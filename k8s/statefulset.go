package k8s

import (
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
)

const (
	eventKilling          = "Killing"
	eventFailedScheduling = "FailedScheduling"
	eventFailedScaleUp    = "NotTriggerScaleUp"
	appSourceType         = "APP"

	AnnotationAppName              = "cloudfoundry.org/application_name"
	AnnotationVersion              = "cloudfoundry.org/version"
	AnnotationAppID                = "cloudfoundry.org/application_id"
	AnnotationSpaceName            = "cloudfoundry.org/space_name"
	AnnotationOrgName              = "cloudfoundry.org/org_name"
	AnnotationOrgGUID              = "cloudfoundry.org/org_guid"
	AnnotationSpaceGUID            = "cloudfoundry.org/space_guid"
	AnnotationLastUpdated          = "cloudfoundry.org/last_updated"
	AnnotationProcessGUID          = "cloudfoundry.org/process_guid"
	AnnotationRegisteredRoutes     = "cloudfoundry.org/routes"
	AnnotationOriginalRequest      = "cloudfoundry.org/original_request"
	AnnotationCompletionCallback   = "cloudfoundry.org/completion_callback"
	AnnotationOpiTaskContainerName = "cloudfoundry.org/opi-task-container-name"
	AnnotationGUID                 = "cloudfoundry.org/guid"

	AnnotationStagingGUID = "cloudfoundry.org/staging_guid"

	LabelGUID           = AnnotationGUID
	LabelName           = "cloudfoundry.org/name"
	LabelVersion        = "cloudfoundry.org/version"
	LabelAppGUID        = "cloudfoundry.org/app_guid"
	LabelProcessType    = "cloudfoundry.org/process_type"
	LabelSourceType     = "cloudfoundry.org/source_type"
	LabelEiriniInstance = "cloudfoundry.org/internal_eirini_instance"

	LabelStagingGUID = "cloudfoundry.org/staging_guid"

	VcapUID                  = 2000
	PdbMinAvailableInstances = 1
	PodAffinityTermWeight    = 100
)

//counterfeiter:generate . PodListerDeleter
type PodListerDeleter interface {
	List(opts metav1.ListOptions) (*corev1.PodList, error)
	Delete(namespace, name string) error
}

//counterfeiter:generate . PodDisruptionBudgetClient
type PodDisruptionBudgetClient interface {
	Create(namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error)
	Delete(namespace string, name string) error
}

//counterfeiter:generate . StatefulSetClient
type StatefulSetClient interface {
	Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*appsv1.StatefulSetList, error)
}

//counterfeiter:generate . SecretsCreatorDeleter
type SecretsCreatorDeleter interface {
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	Delete(namespace string, name string) error
}

//counterfeiter:generate . EventLister
type EventLister interface {
	List(ctx context.Context, opts metav1.ListOptions) (*corev1.EventList, error)
}

//counterfeiter:generate . LRPMapper
type LRPMapper func(s appsv1.StatefulSet) *opi.LRP

type StatefulSetDesirer struct {
	Pods                              PodListerDeleter
	Secrets                           SecretsCreatorDeleter
	StatefulSets                      StatefulSetClient
	PodDisruptionBudets               PodDisruptionBudgetClient
	Events                            EventLister
	StatefulSetToLRPMapper            LRPMapper
	RegistrySecretName                string
	RootfsVersion                     string
	LivenessProbeCreator              ProbeCreator
	ReadinessProbeCreator             ProbeCreator
	Hasher                            util.Hasher
	Logger                            lager.Logger
	ApplicationServiceAccount         string
	AllowAutomountServiceAccountToken bool
}

//counterfeiter:generate . ProbeCreator
type ProbeCreator func(lrp *opi.LRP) *corev1.Probe

type DesirerOption func(resource interface{}) error

func (m *StatefulSetDesirer) Desire(namespace string, lrp *opi.LRP, opts ...DesirerOption) error {
	if lrp.PrivateRegistry != nil {
		secret, err := m.generateRegistryCredsSecret(lrp)
		if err != nil {
			return errors.Wrap(err, "failed to generate private registry secret for statefulset")
		}
		if _, err := m.Secrets.Create(namespace, secret); err != nil {
			return errors.Wrap(err, "failed to create private registry secret for statefulset")
		}
	}

	st := m.toStatefulSet(lrp)
	st.Namespace = namespace
	for _, opt := range opts {
		if err := opt(st); err != nil {
			m.Logger.Error("failed to apply option", err, lager.Data{"guid": lrp.GUID, "version": lrp.Version})
			return errors.Wrap(err, "failed to apply options")
		}
	}

	if _, err := m.StatefulSets.Create(namespace, st); err != nil {
		var statusErr *k8serrors.StatusError
		if errors.As(err, &statusErr) && statusErr.Status().Reason == metav1.StatusReasonAlreadyExists {
			m.Logger.Debug("statefulset already exists", lager.Data{"guid": lrp.GUID, "version": lrp.Version, "error": err.Error()})
			return nil
		}
		return errors.Wrap(err, "failed to create statefulset")
	}

	return m.createPodDisruptionBudget(namespace, lrp)
}

func (m *StatefulSetDesirer) List() ([]*opi.LRP, error) {
	statefulsets, err := m.StatefulSets.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelSourceType, appSourceType),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	return m.statefulSetsToLRPs(statefulsets), nil
}

func (m *StatefulSetDesirer) Stop(identifier opi.LRPIdentifier) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return m.stop(identifier)
	})
	return errors.Wrap(err, "failed to delete statefulset")
}

func (m *StatefulSetDesirer) stop(identifier opi.LRPIdentifier) error {
	log := m.Logger.Session("stop", lager.Data{"guid": identifier.GUID, "version": identifier.Version})
	statefulSet, err := m.getStatefulSet(identifier)
	if errors.Is(err, eirini.ErrNotFound) {
		log.Debug("statefulset does not exist")
		return nil
	}
	if err != nil {
		log.Debug("failed to get statefulset", lager.Data{"error": err.Error()})
		return err
	}

	err = m.PodDisruptionBudets.Delete(statefulSet.Namespace, statefulSet.Name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	err = m.deletePrivateRegistrySecret(statefulSet)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	backgroundPropagation := metav1.DeletePropagationBackground
	return m.StatefulSets.Delete(statefulSet.Namespace, statefulSet.Name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
}

func (m *StatefulSetDesirer) deletePrivateRegistrySecret(statefulSet *appsv1.StatefulSet) error {
	for _, secret := range statefulSet.Spec.Template.Spec.ImagePullSecrets {
		if secret.Name == m.privateRegistrySecretName(statefulSet.Name) {
			return m.Secrets.Delete(statefulSet.Namespace, secret.Name)
		}
	}

	return nil
}

func (m *StatefulSetDesirer) StopInstance(identifier opi.LRPIdentifier, index uint) error {
	log := m.Logger.Session("stopInstance", lager.Data{"guid": identifier.GUID, "version": identifier.Version, "index": index})
	statefulset, err := m.getStatefulSet(identifier)
	if errors.Is(err, eirini.ErrNotFound) {
		log.Debug("statefulset does not exist")
		return nil
	}
	if err != nil {
		log.Debug("failed to get statefulset", lager.Data{"error": err.Error()})
		return err
	}

	if int32(index) >= *statefulset.Spec.Replicas {
		return eirini.ErrInvalidInstanceIndex
	}

	err = m.Pods.Delete(statefulset.Namespace, fmt.Sprintf("%s-%d", statefulset.Name, index))
	if k8serrors.IsNotFound(err) {
		log.Debug("pod does not exist")
		return nil
	}
	return errors.Wrap(err, "failed to delete pod")
}

func (m *StatefulSetDesirer) Update(lrp *opi.LRP) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return m.update(lrp)
	})
	return errors.Wrap(err, "failed to update statefulset")
}

func (m *StatefulSetDesirer) update(lrp *opi.LRP) error {
	statefulSet, err := m.getStatefulSet(opi.LRPIdentifier{GUID: lrp.GUID, Version: lrp.Version})
	if err != nil {
		return err
	}

	count := int32(lrp.TargetInstances)
	statefulSet.Spec.Replicas = &count
	statefulSet.Annotations[AnnotationLastUpdated] = lrp.LastUpdated
	statefulSet.Annotations[AnnotationRegisteredRoutes] = lrp.AppURIs

	_, err = m.StatefulSets.Update(statefulSet.Namespace, statefulSet)
	if err != nil {
		return err
	}

	if lrp.TargetInstances <= 1 {
		err = m.PodDisruptionBudets.Delete(statefulSet.Namespace, statefulSet.Name)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	err = m.createPodDisruptionBudget(statefulSet.Namespace, lrp)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (m *StatefulSetDesirer) Get(identifier opi.LRPIdentifier) (*opi.LRP, error) {
	statefulset, err := m.getStatefulSet(identifier)
	if err != nil {
		return nil, err
	}
	return m.StatefulSetToLRPMapper(*statefulset), nil
}

func (m *StatefulSetDesirer) getStatefulSet(identifier opi.LRPIdentifier) (*appsv1.StatefulSet, error) {
	statefulSet, err := m.StatefulSets.List(metav1.ListOptions{
		LabelSelector: labelSelectorString(identifier),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}
	statefulsets := statefulSet.Items
	switch len(statefulsets) {
	case 0:
		return nil, eirini.ErrNotFound
	case 1:
		return &statefulsets[0], nil
	default:
		panic(fmt.Sprintf("more than one was identified as %+v", identifier))
	}
}

func (m *StatefulSetDesirer) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	if _, err := m.Get(identifier); err == eirini.ErrNotFound {
		return []*opi.Instance{}, err
	}

	pods, err := m.Pods.List(metav1.ListOptions{
		LabelSelector: labelSelectorString(identifier),
	})
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

func (m *StatefulSetDesirer) createPodDisruptionBudget(namespace string, lrp *opi.LRP) error {
	if lrp.TargetInstances > 1 {
		minAvailable := intstr.FromInt(PdbMinAvailableInstances)
		_, err := m.PodDisruptionBudets.Create(namespace, &v1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name: m.statefulSetName(lrp),
			},
			Spec: v1beta1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
				Selector:     m.labelSelector(lrp),
			},
		})
		return err
	}

	return nil
}

func hasInsufficientMemory(eventList *corev1.EventList) bool {
	events := eventList.Items

	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]
	return (event.Reason == eventFailedScheduling || event.Reason == eventFailedScaleUp) &&
		strings.Contains(event.Message, "Insufficient memory")
}

func (m *StatefulSetDesirer) statefulSetsToLRPs(statefulSets *appsv1.StatefulSetList) []*opi.LRP {
	lrps := []*opi.LRP{}
	for _, s := range statefulSets.Items {
		lrp := m.StatefulSetToLRPMapper(s)
		lrps = append(lrps, lrp)
	}
	return lrps
}

func (m *StatefulSetDesirer) statefulSetName(lrp *opi.LRP) string {
	nameSuffix, err := m.Hasher.Hash(fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version))
	if err != nil {
		panic(err)
	}
	namePrefix := fmt.Sprintf("%s-%s", lrp.AppName, lrp.SpaceName)
	namePrefix = utils.SanitizeName(namePrefix, lrp.GUID)

	return fmt.Sprintf("%s-%s", namePrefix, nameSuffix)
}

func (m *StatefulSetDesirer) privateRegistrySecretName(statefulSetName string) string {
	return fmt.Sprintf("%s-registry-credentials", statefulSetName)
}

func (m *StatefulSetDesirer) generateRegistryCredsSecret(lrp *opi.LRP) (*corev1.Secret, error) {
	dockerConfig := dockerutils.NewDockerConfig(
		lrp.PrivateRegistry.Server,
		lrp.PrivateRegistry.Username,
		lrp.PrivateRegistry.Password,
	)

	dockerConfigJSON, err := dockerConfig.JSON()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate privete registry config")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.privateRegistrySecretName(m.statefulSetName(lrp)),
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			dockerutils.DockerConfigKey: dockerConfigJSON,
		},
	}, nil
}

func (m *StatefulSetDesirer) calculateImagePullSecrets(lrp *opi.LRP) []corev1.LocalObjectReference {
	imagePullSecrets := []corev1.LocalObjectReference{
		{Name: m.RegistrySecretName},
	}

	if lrp.PrivateRegistry != nil {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{
			Name: m.privateRegistrySecretName(m.statefulSetName(lrp)),
		})
	}
	return imagePullSecrets
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

	livenessProbe := m.LivenessProbeCreator(lrp)
	readinessProbe := m.ReadinessProbeCreator(lrp)

	memory := *resource.NewScaledQuantity(lrp.MemoryMB, resource.Mega)
	cpu := toCPUMillicores(lrp.CPUWeight)
	ephemeralStorage := *resource.NewScaledQuantity(lrp.DiskMB, resource.Mega)

	volumes, volumeMounts := getVolumeSpecs(lrp.VolumeMounts)
	allowPrivilegeEscalation := false

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.statefulSetName(lrp),
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy: "Parallel",
			Replicas:            int32ptr(lrp.TargetInstances),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: m.calculateImagePullSecrets(lrp),
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
					SecurityContext:    m.getGetSecurityContext(lrp),
					ServiceAccountName: m.ApplicationServiceAccount,
					Volumes:            volumes,
				},
			},
		},
	}

	automountServiceAccountToken := false
	if !m.AllowAutomountServiceAccountToken {
		statefulSet.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	statefulSet.Spec.Selector = m.labelSelector(lrp)

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
		LabelGUID:                        lrp.GUID,
		LabelProcessType:                 lrp.ProcessType,
		LabelVersion:                     lrp.Version,
		LabelAppGUID:                     lrp.AppGUID,
		LabelSourceType:                  appSourceType,
		rootfspatcher.RootfsVersionLabel: m.RootfsVersion,
	}

	statefulSet.Spec.Template.Labels = labels
	statefulSet.Labels = labels

	annotations := map[string]string{
		AnnotationSpaceName:        lrp.SpaceName,
		AnnotationSpaceGUID:        lrp.SpaceGUID,
		AnnotationOriginalRequest:  lrp.LRP,
		AnnotationRegisteredRoutes: lrp.AppURIs,
		AnnotationAppID:            lrp.AppGUID,
		AnnotationVersion:          lrp.Version,
		AnnotationLastUpdated:      lrp.LastUpdated,
		AnnotationProcessGUID:      lrp.ProcessGUID(),
		AnnotationAppName:          lrp.AppName,
		AnnotationOrgName:          lrp.OrgName,
		AnnotationOrgGUID:          lrp.OrgGUID,
	}

	for k, v := range lrp.UserDefinedAnnotations {
		annotations[k] = v
	}

	statefulSet.Annotations = annotations
	statefulSet.Spec.Template.Annotations = annotations
	statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey] = corev1.SeccompProfileRuntimeDefault

	return statefulSet
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

func (m *StatefulSetDesirer) labelSelector(lrp *opi.LRP) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			LabelGUID:       lrp.GUID,
			LabelVersion:    lrp.Version,
			LabelSourceType: appSourceType,
		},
	}
}

func (m *StatefulSetDesirer) getGetSecurityContext(lrp *opi.LRP) *corev1.PodSecurityContext {
	if lrp.RunsAsRoot {
		return nil
	}
	runAsNonRoot := true
	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    int64ptr(VcapUID),
	}
}

func labelSelectorString(id opi.LRPIdentifier) string {
	return fmt.Sprintf(
		"%s=%s,%s=%s",
		LabelGUID, id.GUID,
		LabelVersion, id.Version,
	)
}
