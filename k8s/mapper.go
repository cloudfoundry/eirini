package k8s

import (
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func StatefulSetToLRP(s appsv1.StatefulSet) *opi.LRP {
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
		DiskMB:       disk,
		VolumeMounts: volMounts,
	}
}
