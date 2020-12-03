package k8s

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
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

	AnnotationAppName                        = "cloudfoundry.org/application_name"
	AnnotationVersion                        = "cloudfoundry.org/version"
	AnnotationAppID                          = "cloudfoundry.org/application_id"
	AnnotationSpaceName                      = "cloudfoundry.org/space_name"
	AnnotationOrgName                        = "cloudfoundry.org/org_name"
	AnnotationOrgGUID                        = "cloudfoundry.org/org_guid"
	AnnotationSpaceGUID                      = "cloudfoundry.org/space_guid"
	AnnotationLastUpdated                    = "cloudfoundry.org/last_updated"
	AnnotationProcessGUID                    = "cloudfoundry.org/process_guid"
	AnnotationRegisteredRoutes               = "cloudfoundry.org/routes"
	AnnotationOriginalRequest                = "cloudfoundry.org/original_request"
	AnnotationCompletionCallback             = "cloudfoundry.org/completion_callback"
	AnnotationOpiTaskContainerName           = "cloudfoundry.org/opi-task-container-name"
	AnnotationOpiTaskCompletionReportCounter = "cloudfoundry.org/task_completion_report_counter"
	AnnotationCCAckedTaskCompletion          = "cloudfoundry.org/cc_acked_task_completion"
	AnnotationLastReportedAppCrash           = "cloudfoundry.org/last_reported_app_crash"
	AnnotationLastReportedLRPCrash           = "cloudfoundry.org/last_reported_lrp_crash"
	AnnotationGUID                           = "cloudfoundry.org/guid"

	AppSourceType = "APP"

	LabelGUID        = AnnotationGUID
	LabelOrgGUID     = AnnotationOrgGUID
	LabelOrgName     = AnnotationOrgName
	LabelSpaceGUID   = AnnotationSpaceGUID
	LabelSpaceName   = AnnotationSpaceName
	LabelName        = "cloudfoundry.org/name"
	LabelVersion     = "cloudfoundry.org/version"
	LabelAppGUID     = "cloudfoundry.org/app_guid"
	LabelProcessType = "cloudfoundry.org/process_type"
	LabelSourceType  = "cloudfoundry.org/source_type"

	LabelTaskCompleted = "cloudfoundry.org/task_completed"
	TaskCompletedTrue  = "true"

	OPIContainerName = "opi"

	PdbMinAvailableInstances = 1
	PodAffinityTermWeight    = 100
)

//counterfeiter:generate . PodClient
//counterfeiter:generate . PodDisruptionBudgetClient
//counterfeiter:generate . StatefulSetClient
//counterfeiter:generate . SecretsCreatorDeleter
//counterfeiter:generate . EventsClient
//counterfeiter:generate . LRPMapper
//counterfeiter:generate . ProbeCreator
//counterfeiter:generate . DesireOption
//counterfeiter:generate . StatefulSetClient

type PodClient interface {
	GetAll() ([]corev1.Pod, error)
	GetByLRPIdentifier(opi.LRPIdentifier) ([]corev1.Pod, error)
	Delete(namespace, name string) error
}

type PodDisruptionBudgetClient interface {
	Create(namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error)
	Delete(namespace string, name string) error
}

type StatefulSetClient interface {
	Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Delete(namespace string, name string) error
	GetBySourceType(sourceType string) ([]appsv1.StatefulSet, error)
	GetByLRPIdentifier(id opi.LRPIdentifier) ([]appsv1.StatefulSet, error)
}

type SecretsCreatorDeleter interface {
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	Delete(namespace string, name string) error
}

type EventsClient interface {
	GetByPod(pod corev1.Pod) ([]corev1.Event, error)
}

type LRPMapper func(s appsv1.StatefulSet) (*opi.LRP, error)

type StatefulSetDesirer struct {
	Pods                              PodClient
	Secrets                           SecretsCreatorDeleter
	StatefulSets                      StatefulSetClient
	PodDisruptionBudgets              PodDisruptionBudgetClient
	EventsClient                      EventsClient
	StatefulSetToLRPMapper            LRPMapper
	RegistrySecretName                string
	LivenessProbeCreator              ProbeCreator
	ReadinessProbeCreator             ProbeCreator
	Logger                            lager.Logger
	ApplicationServiceAccount         string
	AllowAutomountServiceAccountToken bool
}

type ProbeCreator func(lrp *opi.LRP) *corev1.Probe

type DesireOption func(resource interface{}) error

func (m *StatefulSetDesirer) Desire(namespace string, lrp *opi.LRP, opts ...DesireOption) error {
	logger := m.Logger.Session("desire", lager.Data{"guid": lrp.GUID, "version": lrp.Version, "namespace": namespace})

	statefulSetName, err := utils.GetStatefulsetName(lrp)
	if err != nil {
		return err
	}

	if lrp.PrivateRegistry != nil {
		err = m.createRegistryCredsSecret(namespace, statefulSetName, lrp)
		if err != nil {
			return err
		}
	}

	st, err := m.toStatefulSet(statefulSetName, lrp)
	if err != nil {
		return err
	}

	st.Namespace = namespace

	err = applyOpts(st, opts...)
	if err != nil {
		return err
	}

	if _, err := m.StatefulSets.Create(namespace, st); err != nil {
		var statusErr *k8serrors.StatusError
		if errors.As(err, &statusErr) && statusErr.Status().Reason == metav1.StatusReasonAlreadyExists {
			logger.Debug("statefulset-already-exists", lager.Data{"error": err.Error()})

			return nil
		}

		return errors.Wrap(err, "failed to create statefulset")
	}

	if err := m.createPodDisruptionBudget(namespace, statefulSetName, lrp); err != nil {
		logger.Error("failed-to-create-pod-disruption-budget", err)

		return errors.Wrap(err, "failed to create pod disruption budget")
	}

	return nil
}

func (m *StatefulSetDesirer) List() ([]*opi.LRP, error) {
	logger := m.Logger.Session("list")

	statefulsets, err := m.StatefulSets.GetBySourceType(AppSourceType)
	if err != nil {
		logger.Error("failed-to-list-statefulsets", err)

		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	lrps, err := m.statefulSetsToLRPs(statefulsets)
	if err != nil {
		logger.Error("failed-to-map-statefulsets-to-lrps", err)

		return nil, errors.Wrap(err, "failed to map statefulsets to lrps")
	}

	return lrps, nil
}

func (m *StatefulSetDesirer) Stop(identifier opi.LRPIdentifier) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return m.stop(identifier)
	})

	return errors.Wrap(err, "failed to delete statefulset")
}

func (m *StatefulSetDesirer) stop(identifier opi.LRPIdentifier) error {
	logger := m.Logger.Session("stop", lager.Data{"guid": identifier.GUID, "version": identifier.Version})
	statefulSet, err := m.getStatefulSet(identifier)

	if errors.Is(err, eirini.ErrNotFound) {
		logger.Debug("statefulset-does-not-exist")

		return nil
	}

	if err != nil {
		logger.Error("failed-to-get-statefulset", err)

		return err
	}

	err = m.PodDisruptionBudgets.Delete(statefulSet.Namespace, statefulSet.Name)
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error("failed-to-delete-disruption-budget", err)

		return errors.Wrap(err, "failed to delete pod disruption budget")
	}

	err = m.deletePrivateRegistrySecret(statefulSet)
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error("failed-to-delete-private-registry-secret", err)

		return err
	}

	if err := m.StatefulSets.Delete(statefulSet.Namespace, statefulSet.Name); err != nil {
		logger.Error("failed-to-delete-statefulset", err)

		return errors.Wrap(err, "failed to delete statefulset")
	}

	return nil
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
	logger := m.Logger.Session("stopInstance", lager.Data{"guid": identifier.GUID, "version": identifier.Version, "index": index})
	statefulset, err := m.getStatefulSet(identifier)

	if errors.Is(err, eirini.ErrNotFound) {
		logger.Debug("statefulset-does-not-exist")

		return nil
	}

	if err != nil {
		logger.Debug("failed-to-get-statefulset", lager.Data{"error": err.Error()})

		return err
	}

	if int32(index) >= *statefulset.Spec.Replicas {
		return eirini.ErrInvalidInstanceIndex
	}

	err = m.Pods.Delete(statefulset.Namespace, fmt.Sprintf("%s-%d", statefulset.Name, index))
	if k8serrors.IsNotFound(err) {
		logger.Debug("pod-does-not-exist")

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
	logger := m.Logger.Session("update", lager.Data{"guid": lrp.GUID, "version": lrp.Version})

	statefulSet, err := m.getStatefulSet(opi.LRPIdentifier{GUID: lrp.GUID, Version: lrp.Version})
	if err != nil {
		logger.Error("failed-to-get-statefulset", err)

		return err
	}

	updatedStatefulSet, err := m.getUpdatedStatefulSetObj(statefulSet,
		lrp.AppURIs,
		lrp.TargetInstances,
		lrp.LastUpdated,
		lrp.Image,
	)
	if err != nil {
		logger.Error("failed-to-get-updated-statefulset", err)

		return err
	}

	_, err = m.StatefulSets.Update(updatedStatefulSet.Namespace, updatedStatefulSet)
	if err != nil {
		logger.Error("failed-to-update-statefulset", err, lager.Data{"namespace": statefulSet.Namespace})

		return errors.Wrap(err, "failed to update statefulset")
	}

	return m.handlePodDisruptionBudget(logger,
		statefulSet.Namespace,
		statefulSet.Name,
		lrp,
	)
}

func (m *StatefulSetDesirer) Get(identifier opi.LRPIdentifier) (*opi.LRP, error) {
	logger := m.Logger.Session("get", lager.Data{"guid": identifier.GUID, "version": identifier.Version})

	return m.getLRP(logger, identifier)
}

func (m *StatefulSetDesirer) getLRP(logger lager.Logger, identifier opi.LRPIdentifier) (*opi.LRP, error) {
	statefulset, err := m.getStatefulSet(identifier)
	if err != nil {
		logger.Error("failed-to-get-statefulset", err)

		return nil, err
	}

	lrp, err := m.StatefulSetToLRPMapper(*statefulset)
	if err != nil {
		logger.Error("failed-to-map-statefulset-to-lrp", err)

		return nil, err
	}

	return lrp, nil
}

func (m *StatefulSetDesirer) getStatefulSet(identifier opi.LRPIdentifier) (*appsv1.StatefulSet, error) {
	statefulSets, err := m.StatefulSets.GetByLRPIdentifier(identifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	switch len(statefulSets) {
	case 0:
		return nil, eirini.ErrNotFound
	case 1:
		return &statefulSets[0], nil
	default:
		return nil, fmt.Errorf("multiple statefulsets found for LRP identifier %+v", identifier)
	}
}

func (m *StatefulSetDesirer) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	logger := m.Logger.Session("get-instance", lager.Data{"guid": identifier.GUID, "version": identifier.Version})
	if _, err := m.getLRP(logger, identifier); errors.Is(err, eirini.ErrNotFound) {
		return nil, err
	}

	pods, err := m.Pods.GetByLRPIdentifier(identifier)
	if err != nil {
		logger.Error("failed-to-list-pods", err)

		return nil, errors.Wrap(err, "failed to list pods")
	}

	instances := []*opi.Instance{}

	for _, pod := range pods {
		events, err := m.EventsClient.GetByPod(pod)
		if err != nil {
			logger.Error("failed-to-get-events", err)

			return nil, errors.Wrapf(err, "failed to get events for pod %s", pod.Name)
		}

		if IsStopped(events) {
			continue
		}

		index, err := util.ParseAppIndex(pod.Name)
		if err != nil {
			logger.Error("failed-to-parse-app-index", err)

			return nil, errors.Wrap(err, "failed to parse pod index")
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

func (m *StatefulSetDesirer) createPodDisruptionBudget(namespace, statefulSetName string, lrp *opi.LRP) error {
	if lrp.TargetInstances > 1 {
		minAvailable := intstr.FromInt(PdbMinAvailableInstances)
		_, err := m.PodDisruptionBudgets.Create(namespace, &v1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name: statefulSetName,
			},
			Spec: v1beta1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
				Selector:     m.labelSelector(lrp),
			},
		})

		return errors.Wrap(err, "failed to create pod distruption budget")
	}

	return nil
}

func hasInsufficientMemory(events []corev1.Event) bool {
	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]

	return (event.Reason == eventFailedScheduling || event.Reason == eventFailedScaleUp) &&
		strings.Contains(event.Message, "Insufficient memory")
}

func (m *StatefulSetDesirer) statefulSetsToLRPs(statefulSets []appsv1.StatefulSet) ([]*opi.LRP, error) {
	lrps := []*opi.LRP{}

	for _, s := range statefulSets {
		lrp, err := m.StatefulSetToLRPMapper(s)
		if err != nil {
			return nil, err
		}

		lrps = append(lrps, lrp)
	}

	return lrps, nil
}

func (m *StatefulSetDesirer) privateRegistrySecretName(statefulSetName string) string {
	return fmt.Sprintf("%s-registry-credentials", statefulSetName)
}

func (m *StatefulSetDesirer) generateRegistryCredsSecret(statefulSetName string, lrp *opi.LRP) (*corev1.Secret, error) {
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
			Name: m.privateRegistrySecretName(statefulSetName),
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			dockerutils.DockerConfigKey: dockerConfigJSON,
		},
	}, nil
}

func (m *StatefulSetDesirer) calculateImagePullSecrets(statefulSetName string, lrp *opi.LRP) []corev1.LocalObjectReference {
	imagePullSecrets := []corev1.LocalObjectReference{
		{Name: m.RegistrySecretName},
	}

	if lrp.PrivateRegistry != nil {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{
			Name: m.privateRegistrySecretName(statefulSetName),
		})
	}

	return imagePullSecrets
}

func (m *StatefulSetDesirer) toStatefulSet(statefulSetName string, lrp *opi.LRP) (*appsv1.StatefulSet, error) { //nolint:funlen // this is a boilerplate function, its length is fine
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

	volumes, volumeMounts := getVolumeSpecs(lrp.VolumeMounts)
	allowPrivilegeEscalation := false
	imagePullSecrets := m.calculateImagePullSecrets(statefulSetName, lrp)

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
	}

	for k, v := range lrp.UserDefinedAnnotations {
		annotations[k] = v
	}

	statefulSet.Annotations = annotations
	statefulSet.Spec.Template.Annotations = annotations
	statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey] = corev1.SeccompProfileRuntimeDefault

	return statefulSet, nil
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

func getSidecarContainers(lrp *opi.LRP) []corev1.Container {
	containers := []corev1.Container{}

	for _, s := range lrp.Sidecars {
		c := corev1.Container{
			Name:      s.Name,
			Command:   s.Command,
			Image:     lrp.Image,
			Env:       MapToEnvVar(s.Env),
			Resources: getContainerResources(lrp.CPUWeight, s.MemoryMB, lrp.DiskMB),
		}
		containers = append(containers, c)
	}

	return containers
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
			LabelSourceType: AppSourceType,
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
	}
}

func (m *StatefulSetDesirer) handlePodDisruptionBudget(logger lager.Logger, namespace, name string, lrp *opi.LRP) error {
	if lrp.TargetInstances <= 1 {
		err := m.PodDisruptionBudgets.Delete(namespace, name)
		if err != nil && !k8serrors.IsNotFound(err) {
			logger.Error("failed-to-delete-disruption-budget", err, lager.Data{"namespace": namespace})

			return errors.Wrap(err, "failed to delete pod disruption budget")
		}

		return nil
	}

	err := m.createPodDisruptionBudget(namespace, name, lrp)

	if err != nil && !k8serrors.IsAlreadyExists(err) {
		logger.Error("failed-to-create-disruption-budget", err, lager.Data{"namespace": namespace})

		return err
	}

	return nil
}

func (m *StatefulSetDesirer) getUpdatedStatefulSetObj(sts *appsv1.StatefulSet, routes []opi.Route, instances int, lastUpdated, image string) (*appsv1.StatefulSet, error) {
	updatedSts := sts.DeepCopy()

	uris, err := json.Marshal(routes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal routes")
	}

	count := int32(instances)
	updatedSts.Spec.Replicas = &count
	updatedSts.Annotations[AnnotationLastUpdated] = lastUpdated
	updatedSts.Annotations[AnnotationRegisteredRoutes] = string(uris)

	if image != "" {
		for i, container := range updatedSts.Spec.Template.Spec.Containers {
			if container.Name == OPIContainerName {
				updatedSts.Spec.Template.Spec.Containers[i].Image = image
			}
		}
	}

	return updatedSts, nil
}

func (m *StatefulSetDesirer) createRegistryCredsSecret(namespace, statefulSetName string, lrp *opi.LRP) error {
	secret, err := m.generateRegistryCredsSecret(statefulSetName, lrp)
	if err != nil {
		return errors.Wrap(err, "failed to generate private registry secret for statefulset")
	}

	_, err = m.Secrets.Create(namespace, secret)

	return errors.Wrap(err, "failed to create private registry secret for statefulset")
}

func applyOpts(statefulset *appsv1.StatefulSet, opts ...DesireOption) error {
	for _, opt := range opts {
		if err := opt(statefulset); err != nil {
			return errors.Wrap(err, "failed to apply options")
		}
	}

	return nil
}
