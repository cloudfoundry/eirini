package stset

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func MapStatefulSetToLRP(s appsv1.StatefulSet) (*opi.LRP, error) {
	stRoutes := s.Annotations[AnnotationRegisteredRoutes]

	var uris []opi.Route

	err := json.Unmarshal([]byte(stRoutes), &uris)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal uris")
	}

	ports := []int32{}
	container := s.Spec.Template.Spec.Containers[0]

	for _, port := range container.Ports {
		ports = append(ports, port.ContainerPort)
	}

	memory := container.Resources.Requests.Memory().ScaledValue(resource.Mega)
	disk := container.Resources.Limits.StorageEphemeral().ScaledValue(resource.Mega)
	volMounts := []opi.VolumeMount{}

	for _, vol := range container.VolumeMounts {
		volMounts = append(volMounts, opi.VolumeMount{
			ClaimName: vol.Name,
			MountPath: vol.MountPath,
		})
	}

	return &opi.LRP{
		LRPIdentifier: opi.LRPIdentifier{
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
		AppURIs:          uris,
		AppGUID:          s.Annotations[AnnotationAppID],
		MemoryMB:         memory,
		DiskMB:           disk,
		VolumeMounts:     volMounts,
	}, nil
}
