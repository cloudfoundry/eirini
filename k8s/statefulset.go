package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
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
}

//go:generate counterfeiter . ProbeCreator
type ProbeCreator func(lrp *opi.LRP) *v1.Probe

func NewStatefulSetDesirer(client kubernetes.Interface, namespace string) opi.Desirer {
	return &StatefulSetDesirer{
		Client:                client,
		Namespace:             namespace,
		LivenessProbeCreator:  CreateLivenessProbe,
		ReadinessProbeCreator: CreateReadinessProbe,
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

func (m *StatefulSetDesirer) getStatefulSet(identifier opi.LRPIdentifier) (*v1beta2.StatefulSet, error) {
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

func getPodState(pod v1.Pod, events *v1.EventList) (string, string) {
	if hasInsufficientMemory(events) {
		return opi.ErrorState, opi.InsufficientMemoryError
	}

	if statusNotAvailable(&pod) || pod.Status.Phase == v1.PodUnknown {
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

func hasInsufficientMemory(eventList *v1.EventList) bool {
	events := eventList.Items

	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]
	return event.Reason == eventFailedScheduling && strings.Contains(event.Message, "Insufficient memory")
}

func statusNotAvailable(pod *v1.Pod) bool {
	return pod.Status.ContainerStatuses == nil || len(pod.Status.ContainerStatuses) == 0
}

func podPending(pod *v1.Pod) bool {
	status := pod.Status.ContainerStatuses[0]
	return pod.Status.Phase == v1.PodPending || (status.State.Running != nil && !status.Ready)
}

func podCrashed(status v1.ContainerStatus) bool {
	return status.State.Waiting != nil || status.State.Terminated != nil
}

func podRunning(status v1.ContainerStatus) bool {
	return status.State.Running != nil && status.Ready
}

func (m *StatefulSetDesirer) statefulSets() types.StatefulSetInterface {
	return m.Client.AppsV1beta2().StatefulSets(m.Namespace)
}

func statefulSetsToLRPs(statefulSets *v1beta2.StatefulSetList) []*opi.LRP {
	lrps := []*opi.LRP{}
	for _, s := range statefulSets.Items {
		lrp := statefulSetToLRP(s)
		lrps = append(lrps, lrp)
	}
	return lrps
}

func statefulSetToLRP(s v1beta2.StatefulSet) *opi.LRP {
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
		AppName:          s.Labels[cf.VcapAppName],
		SpaceName:        s.Labels[cf.VcapSpaceName],
		Image:            container.Image,
		Command:          container.Command,
		RunningInstances: int(s.Status.ReadyReplicas),
		Ports:            ports,
		Metadata: map[string]string{
			cf.ProcessGUID:          s.Annotations[cf.ProcessGUID],
			cf.LastUpdated:          s.Annotations[cf.LastUpdated],
			cf.VcapAppUris:          s.Annotations[cf.VcapAppUris],
			eirini.RegisteredRoutes: s.Annotations[cf.VcapAppUris],
			cf.VcapAppID:            s.Annotations[cf.VcapAppID],
			cf.VcapVersion:          s.Annotations[cf.VcapVersion],
		},
		MemoryMB:     memory,
		VolumeMounts: volMounts,
	}
}

func (m *StatefulSetDesirer) toStatefulSet(lrp *opi.LRP) *v1beta2.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	fieldEnvs := []v1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "CF_INSTANCE_IP",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: "CF_INSTANCE_INTERNAL_IP",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
	}

	envs = append(envs, fieldEnvs...)
	ports := []v1.ContainerPort{}
	for _, port := range lrp.Ports {
		ports = append(ports, v1.ContainerPort{ContainerPort: port})
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
	namePrefix := util.TruncateString(fmt.Sprintf("%s-%s", lrp.AppName, lrp.SpaceName), 40)
	statefulSet := &v1beta2.StatefulSet{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", namePrefix),
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: int32ptr(lrp.TargetInstances),
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Annotations: map[string]string{
						cf.ProcessGUID: lrp.Metadata[cf.ProcessGUID],
						cf.VcapAppID:   lrp.Metadata[cf.VcapAppID],
					},
				},
				Spec: v1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []v1.Container{
						{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     envs,
							Ports:   ports,
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceMemory: memory,
								},
								Requests: v1.ResourceList{
									v1.ResourceMemory: memory,
									v1.ResourceCPU:    cpu,
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
		"guid":           lrp.GUID,
		"version":        lrp.Version,
		"source_type":    appSourceType,
		cf.VcapSpaceName: lrp.SpaceName,
		cf.VcapAppName:   lrp.AppName,
	}

	statefulSet.Spec.Template.Labels = labels

	statefulSet.Spec.Selector = &meta.LabelSelector{
		MatchLabels: labels,
	}

	statefulSet.Labels = labels

	statefulSet.Annotations = lrp.Metadata
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]

	return statefulSet
}

func getVolumeSpecs(lrpVolumeMounts []opi.VolumeMount) ([]v1.Volume, []v1.VolumeMount) {
	volumes := []v1.Volume{}
	volumeMounts := []v1.VolumeMount{}
	for _, vm := range lrpVolumeMounts {
		volumes = append(volumes, v1.Volume{
			Name: vm.ClaimName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: vm.ClaimName,
				},
			},
		})
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      vm.ClaimName,
			MountPath: vm.MountPath,
		})
	}
	return volumes, volumeMounts
}
