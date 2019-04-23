package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/apps/v1"
)

const (
	eventKilling          = "Killing"
	eventFailedScheduling = "FailedScheduling"
	appSourceType         = "APP"
)

type StatefulSetDesirer struct {
	Client                kubernetes.Interface
	Namespace             string
	LivenessProbeCreator  ProbeCreator
	ReadinessProbeCreator ProbeCreator
	Hasher                util.Hasher
}

//go:generate counterfeiter . ProbeCreator
type ProbeCreator func(lrp *opi.LRP) *corev1.Probe

func NewStatefulSetDesirer(client kubernetes.Interface, namespace string) opi.Desirer {
	return &StatefulSetDesirer{
		Client:                client,
		Namespace:             namespace,
		LivenessProbeCreator:  CreateLivenessProbe,
		ReadinessProbeCreator: CreateReadinessProbe,
		Hasher:                util.TruncatedSHA256Hasher{},
	}
}

func (m *StatefulSetDesirer) List() ([]*opi.LRP, error) {
	statefulsets, err := m.statefulSets().List(meta.ListOptions{})
	if err != nil {
		return nil, err
	}

	lrps := statefulSetsToLRPs(statefulsets)

	return lrps, nil
}

func (m *StatefulSetDesirer) Stop(identifier opi.LRPIdentifier) error {
	statefulSet, err := m.getStatefulSet(identifier)
	if err != nil {
		return err
	}

	backgroundPropagation := meta.DeletePropagationBackground
	return m.statefulSets().Delete(statefulSet.Name, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation})
}

func (m *StatefulSetDesirer) StopInstance(identifier opi.LRPIdentifier, index uint) error {
	selector := fmt.Sprintf("guid=%s,version=%s", identifier.GUID, identifier.Version)
	options := meta.ListOptions{LabelSelector: selector}
	statefulsets, err := m.statefulSets().List(options)
	if err != nil {
		return errors.Wrap(err, "failed to get statefulset")
	}
	if len(statefulsets.Items) == 0 {
		return errors.New("app does not exist")
	}

	st := statefulsets.Items[0]
	return m.Client.CoreV1().Pods(m.Namespace).Delete(fmt.Sprintf("%s-%d", st.Name, index), nil)
}

func (m *StatefulSetDesirer) Desire(lrp *opi.LRP) error {
	_, err := m.statefulSets().Create(m.toStatefulSet(lrp))
	return err
}

func (m *StatefulSetDesirer) Update(lrp *opi.LRP) error {
	statefulSet, err := m.getStatefulSet(opi.LRPIdentifier{GUID: lrp.GUID, Version: lrp.Version})
	if err != nil {
		return err
	}

	count := int32(lrp.TargetInstances)
	statefulSet.Spec.Replicas = &count
	statefulSet.Annotations[cf.LastUpdated] = lrp.Metadata[cf.LastUpdated]
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]

	_, err = m.statefulSets().Update(statefulSet)
	return err
}

func (m *StatefulSetDesirer) Get(identifier opi.LRPIdentifier) (*opi.LRP, error) {
	statefulset, err := m.getStatefulSet(identifier)
	if err != nil {
		return nil, err
	}
	return statefulSetToLRP(*statefulset), nil
}

func (m *StatefulSetDesirer) getStatefulSet(identifier opi.LRPIdentifier) (*appsv1.StatefulSet, error) {
	options := meta.ListOptions{LabelSelector: fmt.Sprintf("guid=%s,version=%s", identifier.GUID, identifier.Version)}
	statefulSet, err := m.statefulSets().List(options)
	if err != nil {
		return nil, err
	}
	statefulsets := statefulSet.Items
	switch len(statefulsets) {
	case 0:
		return nil, errors.New("app not found")
	case 1:
		return &statefulsets[0], nil
	default:
		panic(fmt.Sprintf("more than one was identified as %+v", identifier))
	}
}

func (m *StatefulSetDesirer) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	options := meta.ListOptions{LabelSelector: fmt.Sprintf("guid=%s,version=%s", identifier.GUID, identifier.Version)}
	pods, err := m.Client.CoreV1().Pods(m.Namespace).List(options)
	if err != nil {
		return []*opi.Instance{}, err
	}

	instances := []*opi.Instance{}
	for _, pod := range pods.Items {
		events, err := GetEvents(m.Client, pod)
		if err != nil {
			return []*opi.Instance{}, err
		}

		if IsStopped(events) {
			continue
		}

		_, index, err := util.ParseAppNameAndIndex(pod.Name)
		if err != nil {
			return []*opi.Instance{}, err
		}

		since := int64(0)
		if pod.Status.StartTime != nil {
			since = pod.Status.StartTime.UnixNano()
		}

		state, placementError := getPodState(pod, events)

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

func getPodState(pod corev1.Pod, events *corev1.EventList) (string, string) {
	if hasInsufficientMemory(events) {
		return opi.ErrorState, opi.InsufficientMemoryError
	}

	if statusNotAvailable(&pod) || pod.Status.Phase == corev1.PodUnknown {
		return opi.UnknownState, ""
	}

	if podPending(&pod) {
		return opi.PendingState, ""
	}

	if podCrashed(pod.Status.ContainerStatuses[0]) {
		return opi.CrashedState, ""
	}

	if podRunning(pod.Status.ContainerStatuses[0]) {
		return opi.RunningState, ""
	}

	return opi.UnknownState, ""
}

func hasInsufficientMemory(eventList *corev1.EventList) bool {
	events := eventList.Items

	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]
	return event.Reason == eventFailedScheduling && strings.Contains(event.Message, "Insufficient memory")
}

func statusNotAvailable(pod *corev1.Pod) bool {
	return pod.Status.ContainerStatuses == nil || len(pod.Status.ContainerStatuses) == 0
}

func podPending(pod *corev1.Pod) bool {
	status := pod.Status.ContainerStatuses[0]
	return pod.Status.Phase == corev1.PodPending || (status.State.Running != nil && !status.Ready)
}

func podCrashed(status corev1.ContainerStatus) bool {
	return status.State.Waiting != nil || status.State.Terminated != nil
}

func podRunning(status corev1.ContainerStatus) bool {
	return status.State.Running != nil && status.Ready
}

func (m *StatefulSetDesirer) statefulSets() types.StatefulSetInterface {
	return m.Client.AppsV1().StatefulSets(m.Namespace)
}

func statefulSetsToLRPs(statefulSets *appsv1.StatefulSetList) []*opi.LRP {
	lrps := []*opi.LRP{}
	for _, s := range statefulSets.Items {
		lrp := statefulSetToLRP(s)
		lrps = append(lrps, lrp)
	}
	return lrps
}

func statefulSetToLRP(s appsv1.StatefulSet) *opi.LRP {
	ports := []int32{}
	container := s.Spec.Template.Spec.Containers[0]

	for _, port := range container.Ports {
		ports = append(ports, port.ContainerPort)
	}

	memory := container.Resources.Requests.Memory().ScaledValue(resource.Mega)
	volMounts := []opi.VolumeMount{}
	for _, vol := range container.VolumeMounts {
		volMounts = append(volMounts, opi.VolumeMount{
			ClaimName: vol.Name,
			MountPath: vol.MountPath,
		})
	}

	return &opi.LRP{
		LRPIdentifier: opi.LRPIdentifier{
			GUID:    s.Annotations[cf.VcapAppID],
			Version: s.Annotations[cf.VcapVersion],
		},
		AppName:          s.Annotations[cf.VcapAppName],
		SpaceName:        s.Annotations[cf.VcapSpaceName],
		Image:            container.Image,
		Command:          container.Command,
		RunningInstances: int(s.Status.ReadyReplicas),
		Ports:            ports,
		Metadata: map[string]string{
			cf.ProcessGUID: s.Annotations[cf.ProcessGUID],
			cf.LastUpdated: s.Annotations[cf.LastUpdated],
			cf.VcapAppUris: s.Annotations[cf.VcapAppUris],
			cf.VcapAppID:   s.Annotations[cf.VcapAppID],
			cf.VcapVersion: s.Annotations[cf.VcapVersion],
			cf.VcapAppName: s.Annotations[cf.VcapAppName],
		},
		MemoryMB:     memory,
		VolumeMounts: volMounts,
	}
}

func (m *StatefulSetDesirer) toStatefulSet(lrp *opi.LRP) *appsv1.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	fieldEnvs := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "CF_INSTANCE_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: "CF_INSTANCE_INTERNAL_IP",
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

	memory, err := resource.ParseQuantity(fmt.Sprintf("%dM", lrp.MemoryMB))
	if err != nil {
		panic(err)
	}

	cpu, err := resource.ParseQuantity(fmt.Sprintf("%dm", lrp.CPUWeight*10))
	if err != nil {
		panic(err)
	}

	volumes, volumeMounts := getVolumeSpecs(lrp.VolumeMounts)
	automountServiceAccountToken := false

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
						cf.ProcessGUID: lrp.Metadata[cf.ProcessGUID],
						cf.VcapAppID:   lrp.Metadata[cf.VcapAppID],
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{
						{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     envs,
							Ports:   ports,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: memory,
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

	labels := map[string]string{
		"guid":        lrp.GUID,
		"version":     lrp.Version,
		"source_type": appSourceType,
	}

	statefulSet.Spec.Template.Labels = labels

	statefulSet.Spec.Selector = &meta.LabelSelector{
		MatchLabels: labels,
	}

	statefulSet.Labels = labels

	statefulSet.Annotations = lrp.Metadata
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]
	statefulSet.Annotations[cf.VcapSpaceName] = lrp.SpaceName

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
