package stset

import (
	"code.cloudfoundry.org/eirini/api"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type StatefulSetToLRP func(s appsv1.StatefulSet) (*api.LRP, error)

func NewStatefulSetToLRPConverter() StatefulSetToLRP {
	return MapStatefulSetToLRP
}

func (f StatefulSetToLRP) Convert(s appsv1.StatefulSet) (*api.LRP, error) {
	return f(s)
}

func MapStatefulSetToLRP(s appsv1.StatefulSet) (*api.LRP, error) {
	ports := []int32{}
	container := s.Spec.Template.Spec.Containers[0]

	for _, port := range container.Ports {
		ports = append(ports, port.ContainerPort)
	}

	memory := container.Resources.Requests.Memory().ScaledValue(resource.Mega)
	disk := container.Resources.Limits.StorageEphemeral().ScaledValue(resource.Mega)
	volMounts := []api.VolumeMount{}

	for _, vol := range container.VolumeMounts {
		volMounts = append(volMounts, api.VolumeMount{
			ClaimName: vol.Name,
			MountPath: vol.MountPath,
		})
	}

	return &api.LRP{
		LRPIdentifier: api.LRPIdentifier{
			GUID:    s.Labels[LabelGUID],
			Version: s.Annotations[AnnotationVersion],
		},
		AppName:          s.Annotations[AnnotationAppName],
		SpaceName:        s.Annotations[AnnotationSpaceName],
		Image:            container.Image,
		Command:          container.Command,
		RunningInstances: int(s.Status.ReadyReplicas),
		TargetInstances:  int(*s.Spec.Replicas),
		Ports:            ports,
		LastUpdated:      s.Annotations[AnnotationLastUpdated],
		AppGUID:          s.Annotations[AnnotationAppID],
		MemoryMB:         memory,
		DiskMB:           disk,
		VolumeMounts:     volMounts,
	}, nil
}
