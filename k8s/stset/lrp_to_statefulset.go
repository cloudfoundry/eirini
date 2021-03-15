package stset

import (
	"encoding/json"
	"strconv"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//counterfeiter:generate . ProbeCreator

const PodAffinityTermWeight = 100

type ProbeCreator func(lrp *opi.LRP) *corev1.Probe

type LRPToStatefulSet struct {
	applicationServiceAccount         string
	registrySecretName                string
	allowAutomountServiceAccountToken bool
	allowRunImageAsRoot               bool
	latestMigration                   int
	livenessProbeCreator              ProbeCreator
	readinessProbeCreator             ProbeCreator
}

func NewLRPToStatefulSetConverter(
	applicationServiceAccount string,
	registrySecretName string,
	allowAutomountServiceAccountToken bool,
	allowRunImageAsRoot bool,
	latestMigration int,
	livenessProbeCreator ProbeCreator,
	readinessProbeCreator ProbeCreator,
) *LRPToStatefulSet {
	return &LRPToStatefulSet{
		applicationServiceAccount:         applicationServiceAccount,
		registrySecretName:                registrySecretName,
		allowAutomountServiceAccountToken: allowAutomountServiceAccountToken,
		allowRunImageAsRoot:               allowRunImageAsRoot,
		latestMigration:                   latestMigration,
		livenessProbeCreator:              livenessProbeCreator,
		readinessProbeCreator:             readinessProbeCreator,
	}
}

func (c *LRPToStatefulSet) Convert(statefulSetName string, lrp *opi.LRP) (*appsv1.StatefulSet, error) {
	envs := shared.MapToEnvVar(lrp.Env)
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
			Name: eirini.EnvCFInstanceGUID,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.uid",
				},
			},
		},
		{
			Name: eirini.EnvCFInstanceIP,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.hostIP",
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

	livenessProbe := c.livenessProbeCreator(lrp)
	readinessProbe := c.readinessProbeCreator(lrp)

	volumes, volumeMounts := getVolumeSpecs(lrp.VolumeMounts)
	allowPrivilegeEscalation := false
	imagePullSecrets := c.calculateImagePullSecrets(statefulSetName, lrp)

	containers := []corev1.Container{
		{
			Name:            OPIContainerName,
			Image:           lrp.Image,
			ImagePullPolicy: corev1.PullAlways,
			Command:         lrp.Command,
			Env:             envs,
			Ports:           ports,
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			},
			Resources:      getContainerResources(lrp.CPUWeight, lrp.MemoryMB, lrp.DiskMB),
			LivenessProbe:  livenessProbe,
			ReadinessProbe: readinessProbe,
			VolumeMounts:   volumeMounts,
		},
	}

	sidecarContainers := getSidecarContainers(lrp)
	containers = append(containers, sidecarContainers...)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: statefulSetName,
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy: "Parallel",
			Replicas:            int32ptr(lrp.TargetInstances),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:         containers,
					ImagePullSecrets:   imagePullSecrets,
					SecurityContext:    c.getGetSecurityContext(lrp),
					ServiceAccountName: c.applicationServiceAccount,
					Volumes:            volumes,
				},
			},
		},
	}

	if !c.allowAutomountServiceAccountToken {
		automountServiceAccountToken := false
		statefulSet.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	statefulSet.Spec.Selector = StatefulSetLabelSelector(lrp)

	statefulSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: PodAffinityTermWeight,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: toLabelSelectorRequirements(statefulSet.Spec.Selector),
						},
					},
				},
			},
		},
	}

	labels := map[string]string{
		LabelOrgGUID:     lrp.OrgGUID,
		LabelOrgName:     lrp.OrgName,
		LabelSpaceGUID:   lrp.SpaceGUID,
		LabelSpaceName:   lrp.SpaceName,
		LabelGUID:        lrp.GUID,
		LabelProcessType: lrp.ProcessType,
		LabelVersion:     lrp.Version,
		LabelAppGUID:     lrp.AppGUID,
		LabelSourceType:  AppSourceType,
	}

	statefulSet.Spec.Template.Labels = labels
	statefulSet.Labels = labels

	uris, err := json.Marshal(lrp.AppURIs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal app uris")
	}

	annotations := map[string]string{
		AnnotationSpaceName:        lrp.SpaceName,
		AnnotationSpaceGUID:        lrp.SpaceGUID,
		AnnotationOriginalRequest:  lrp.LRP,
		AnnotationRegisteredRoutes: string(uris),
		AnnotationAppID:            lrp.AppGUID,
		AnnotationVersion:          lrp.Version,
		AnnotationLastUpdated:      lrp.LastUpdated,
		AnnotationProcessGUID:      lrp.ProcessGUID(),
		AnnotationAppName:          lrp.AppName,
		AnnotationOrgName:          lrp.OrgName,
		AnnotationOrgGUID:          lrp.OrgGUID,
		AnnotationLatestMigration:  strconv.Itoa(c.latestMigration),
	}

	for k, v := range lrp.UserDefinedAnnotations {
		annotations[k] = v
	}

	statefulSet.Annotations = annotations
	statefulSet.Spec.Template.Annotations = annotations
	statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey] = corev1.SeccompProfileRuntimeDefault

	return statefulSet, nil
}

func (c *LRPToStatefulSet) calculateImagePullSecrets(statefulSetName string, lrp *opi.LRP) []corev1.LocalObjectReference {
	imagePullSecrets := []corev1.LocalObjectReference{
		{Name: c.registrySecretName},
	}

	if lrp.PrivateRegistry != nil {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{
			Name: privateRegistrySecretName(statefulSetName),
		})
	}

	return imagePullSecrets
}

func (c *LRPToStatefulSet) getGetSecurityContext(lrp *opi.LRP) *corev1.PodSecurityContext {
	if c.allowRunImageAsRoot {
		return nil
	}

	runAsNonRoot := true

	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
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

func getContainerResources(cpuWeight uint8, memoryMB, diskMB int64) corev1.ResourceRequirements {
	memory := *resource.NewScaledQuantity(memoryMB, resource.Mega)
	cpu := toCPUMillicores(cpuWeight)
	ephemeralStorage := *resource.NewScaledQuantity(diskMB, resource.Mega)

	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory:           memory,
			corev1.ResourceEphemeralStorage: ephemeralStorage,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: memory,
			corev1.ResourceCPU:    cpu,
		},
	}
}

func toCPUMillicores(cpuPercentage uint8) resource.Quantity {
	return *resource.NewScaledQuantity(int64(cpuPercentage)*10, resource.Milli) //nolint:gomnd
}

func getSidecarContainers(lrp *opi.LRP) []corev1.Container {
	containers := []corev1.Container{}

	for _, s := range lrp.Sidecars {
		c := corev1.Container{
			Name:      s.Name,
			Command:   s.Command,
			Image:     lrp.Image,
			Env:       shared.MapToEnvVar(s.Env),
			Resources: getContainerResources(lrp.CPUWeight, s.MemoryMB, lrp.DiskMB),
		}
		containers = append(containers, c)
	}

	return containers
}

func int32ptr(i int) *int32 {
	u := int32(i)

	return &u
}

func toLabelSelectorRequirements(selector *metav1.LabelSelector) []metav1.LabelSelectorRequirement {
	labels := selector.MatchLabels
	reqs := make([]metav1.LabelSelectorRequirement, 0, len(labels))

	for label, value := range labels {
		reqs = append(reqs, metav1.LabelSelectorRequirement{
			Key:      label,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{value},
		})
	}

	return reqs
}

func StatefulSetLabelSelector(lrp *opi.LRP) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			LabelGUID:       lrp.GUID,
			LabelVersion:    lrp.Version,
			LabelSourceType: AppSourceType,
		},
	}
}
