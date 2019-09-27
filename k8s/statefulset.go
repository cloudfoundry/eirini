package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	eventKilling          = "Killing"
	eventFailedScheduling = "FailedScheduling"
	appSourceType         = "APP"
)

//go:generate counterfeiter . PodListerDeleter
type PodListerDeleter interface {
	List(opts metav1.ListOptions) (*v1.PodList, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

//go:generate counterfeiter . StatefulSetClient
type StatefulSetClient interface {
	Create(*appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Update(*appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*appsv1.StatefulSetList, error)
}

//go:generate counterfeiter . LRPMapper
type LRPMapper func(s appsv1.StatefulSet) *opi.LRP

type StatefulSetDesirer struct {
	Pods                   PodListerDeleter
	StatefulSets           StatefulSetClient
	Events                 EventLister
	StatefulSetToLRPMapper LRPMapper
	RegistrySecretName     string
	RootfsVersion          string
	LivenessProbeCreator   ProbeCreator
	ReadinessProbeCreator  ProbeCreator
	Hasher                 util.Hasher
	Logger                 lager.Logger
}

var ErrNotFound = errors.New("statefulset not found")

//go:generate counterfeiter . ProbeCreator
type ProbeCreator func(lrp *opi.LRP) *corev1.Probe

func NewStatefulSetDesirer(client kubernetes.Interface, namespace, registrySecretName, rootfsVersion string, logger lager.Logger) opi.Desirer {
	return &StatefulSetDesirer{
		Pods:                   client.CoreV1().Pods(namespace),
		StatefulSets:           client.AppsV1().StatefulSets(namespace),
		Events:                 client.CoreV1().Events(namespace),
		RegistrySecretName:     registrySecretName,
		StatefulSetToLRPMapper: StatefulSetToLRP,
		RootfsVersion:          rootfsVersion,
		LivenessProbeCreator:   CreateLivenessProbe,
		ReadinessProbeCreator:  CreateReadinessProbe,
		Hasher:                 util.TruncatedSHA256Hasher{},
		Logger:                 logger,
	}
}

func (m *StatefulSetDesirer) Desire(lrp *opi.LRP) error {
	_, err := m.StatefulSets.Create(m.toStatefulSet(lrp))
	return errors.Wrap(err, "failed to create statefulset")
}

func (m *StatefulSetDesirer) List() ([]*opi.LRP, error) {
	statefulsets, err := m.StatefulSets.List(meta.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	return m.statefulSetsToLRPs(statefulsets), nil
}

func (m *StatefulSetDesirer) Stop(identifier opi.LRPIdentifier) error {
	statefulSet, err := m.getStatefulSet(identifier)
	if err != nil {
		if err != ErrNotFound {
			return err
		}
		m.Logger.Info("stateful set not found", lager.Data{"guid": identifier.GUID, "version": identifier.Version})
		return nil
	}

	backgroundPropagation := meta.DeletePropagationBackground
	return errors.Wrap(m.StatefulSets.Delete(statefulSet.Name, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation}), "failed to delete statefulset")
}

func (m *StatefulSetDesirer) StopInstance(identifier opi.LRPIdentifier, index uint) error {
	selector := fmt.Sprintf("guid=%s,version=%s", identifier.GUID, identifier.Version)
	options := meta.ListOptions{LabelSelector: selector}
	statefulsets, err := m.StatefulSets.List(options)
	if err != nil {
		return errors.Wrap(err, "failed to get statefulset")
	}
	if len(statefulsets.Items) == 0 {
		return errors.New("app does not exist")
	}

	name := statefulsets.Items[0].Name
	err = m.Pods.Delete(fmt.Sprintf("%s-%d", name, index), nil)
	return errors.Wrap(err, "failed to delete pod")
}

func (m *StatefulSetDesirer) Update(lrp *opi.LRP) error {
	statefulSet, err := m.getStatefulSet(opi.LRPIdentifier{GUID: lrp.GUID, Version: lrp.Version})
	if err != nil {
		return errors.Wrap(err, "failed to get statefulset")
	}

	count := int32(lrp.TargetInstances)
	statefulSet.Spec.Replicas = &count
	statefulSet.Annotations[cf.LastUpdated] = lrp.Metadata[cf.LastUpdated]
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]

	_, err = m.StatefulSets.Update(statefulSet)
	return errors.Wrap(err, "failed to update statefulset")
}

func (m *StatefulSetDesirer) Get(identifier opi.LRPIdentifier) (*opi.LRP, error) {
	statefulset, err := m.getStatefulSet(identifier)
	if err != nil {
		return nil, err
	}
	return m.StatefulSetToLRPMapper(*statefulset), nil
}

func (m *StatefulSetDesirer) getStatefulSet(identifier opi.LRPIdentifier) (*appsv1.StatefulSet, error) {
	options := meta.ListOptions{LabelSelector: fmt.Sprintf("guid=%s,version=%s", identifier.GUID, identifier.Version)}
	statefulSet, err := m.StatefulSets.List(options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}
	statefulsets := statefulSet.Items
	switch len(statefulsets) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &statefulsets[0], nil
	default:
		panic(fmt.Sprintf("more than one was identified as %+v", identifier))
	}
}

func (m *StatefulSetDesirer) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	options := meta.ListOptions{LabelSelector: fmt.Sprintf("guid=%s,version=%s", identifier.GUID, identifier.Version)}
	pods, err := m.Pods.List(options)
	if err != nil {
		return []*opi.Instance{}, errors.Wrap(err, "failed to list pods")
	}

	instances := []*opi.Instance{}
	for _, pod := range pods.Items {
		events, err := GetEvents(m.Events, pod)
		if err != nil {
			return []*opi.Instance{}, errors.Wrapf(err, "failed to get events for pod %s", pod.Name)
		}

		if IsStopped(events) {
			continue
		}

		index, err := util.ParseAppIndex(pod.Name)
		if err != nil {
			return []*opi.Instance{}, err
		}

		since := int64(0)
		if pod.Status.StartTime != nil {
			since = pod.Status.StartTime.UnixNano()
		}

		var state, placementError string
		if hasInsufficientMemory(events) {
			state, placementError = opi.ErrorState, opi.InsufficientMemoryError
		} else {
			state = utils.GetPodState(pod)
		}

		instance := opi.Instance{
			Since:          since,
			Index:          index,
			State:          state,
			PlacementError: placementError,
		}
		instances = append(instances, &instance)
	}

	return instances, nil
}

func hasInsufficientMemory(eventList *corev1.EventList) bool {
	events := eventList.Items

	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]
	return event.Reason == eventFailedScheduling && strings.Contains(event.Message, "Insufficient memory")
}

func (m *StatefulSetDesirer) statefulSetsToLRPs(statefulSets *appsv1.StatefulSetList) []*opi.LRP {
	lrps := []*opi.LRP{}
	for _, s := range statefulSets.Items {
		lrp := m.StatefulSetToLRPMapper(s)
		lrps = append(lrps, lrp)
	}
	return lrps
}

func (m *StatefulSetDesirer) toStatefulSet(lrp *opi.LRP) *appsv1.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	fieldEnvs := []corev1.EnvVar{
		{
			Name: eirini.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: eirini.EnvCFInstanceIP,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: eirini.EnvCFInstanceInternalIP,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
	}

	envs = append(envs, fieldEnvs...)
	ports := []corev1.ContainerPort{}
	for _, port := range lrp.Ports {
		ports = append(ports, corev1.ContainerPort{ContainerPort: port})
	}

	livenessProbe := m.LivenessProbeCreator(lrp)
	readinessProbe := m.ReadinessProbeCreator(lrp)

	memory := *resource.NewScaledQuantity(lrp.MemoryMB, resource.Mega)
	cpu := *resource.NewScaledQuantity(int64(lrp.CPUWeight*10), resource.Milli)
	ephemeralStorage := *resource.NewScaledQuantity(lrp.DiskMB, resource.Mega)

	volumes, volumeMounts := getVolumeSpecs(lrp.VolumeMounts)
	automountServiceAccountToken := false
	allowPrivilegeEscalation := false

	nameSuffix, err := m.Hasher.Hash(fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version))
	if err != nil {
		panic(err)
	}
	namePrefix := fmt.Sprintf("%s-%s", lrp.AppName, lrp.SpaceName)
	namePrefix = utils.SanitizeName(namePrefix, lrp.GUID)

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", namePrefix, nameSuffix),
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy: "Parallel",
			Replicas:            int32ptr(lrp.TargetInstances),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Annotations: map[string]string{
						cf.ProcessGUID:                 lrp.Metadata[cf.ProcessGUID],
						cf.VcapAppID:                   lrp.Metadata[cf.VcapAppID],
						corev1.SeccompPodAnnotationKey: corev1.SeccompProfileRuntimeDefault,
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: m.RegistrySecretName},
					},
					Containers: []corev1.Container{
						{
							Name:            "opi",
							Image:           lrp.Image,
							ImagePullPolicy: corev1.PullAlways,
							Command:         lrp.Command,
							Env:             envs,
							Ports:           ports,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory:           memory,
									corev1.ResourceEphemeralStorage: ephemeralStorage,
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: memory,
									corev1.ResourceCPU:    cpu,
								},
							},
							LivenessProbe:  livenessProbe,
							ReadinessProbe: readinessProbe,
							VolumeMounts:   volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	selectorLabels := map[string]string{
		"guid":        lrp.GUID,
		"version":     lrp.Version,
		"source_type": appSourceType,
	}

	statefulSet.Spec.Selector = &meta.LabelSelector{
		MatchLabels: selectorLabels,
	}

	labels := map[string]string{
		"guid":                           lrp.GUID,
		"process_type":                   lrp.ProcessType,
		"version":                        lrp.Version,
		"app_guid":                       lrp.AppGUID,
		"source_type":                    appSourceType,
		rootfspatcher.RootfsVersionLabel: m.RootfsVersion,
	}

	statefulSet.Spec.Template.Labels = labels
	statefulSet.Labels = labels

	statefulSet.Annotations = lrp.Metadata
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]
	statefulSet.Annotations[cf.VcapSpaceName] = lrp.SpaceName
	statefulSet.Annotations[eirini.OriginalRequest] = lrp.LRP

	return statefulSet
}

func getVolumeSpecs(lrpVolumeMounts []opi.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	for _, vm := range lrpVolumeMounts {
		volumes = append(volumes, corev1.Volume{
			Name: vm.ClaimName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: vm.ClaimName,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vm.ClaimName,
			MountPath: vm.MountPath,
		})
	}
	return volumes, volumeMounts
}
