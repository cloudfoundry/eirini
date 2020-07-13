package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/lager"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
)

const (
	stagingSourceType    = "STG"
	taskSourceType       = "TASK"
	opiTaskContainerName = "opi-task"
	parallelism          = 1
	completions          = 1
)

//counterfeiter:generate . JobClient
type JobClient interface {
	Create(namespace string, job *batch.Job) (*batch.Job, error)
	Update(namespace string, job *batch.Job) (*batch.Job, error)
	Delete(namespace string, name string, options *meta_v1.DeleteOptions) error
	List(opts meta_v1.ListOptions) (*batch.JobList, error)
}

type KeyPath struct {
	Key  string
	Path string
}
type StagingConfigTLS struct {
	SecretName string
	KeyPaths   []KeyPath
}

type TaskDesirer struct {
	logger                    lager.Logger
	jobClient                 JobClient
	secretsClient             SecretsCreatorDeleter
	defaultStagingNamespace   string
	tlsConfig                 []StagingConfigTLS
	serviceAccountName        string
	stagingServiceAccountName string
	registrySecretName        string
	eiriniInstance            string
	rootfsVersion             string
}

func NewTaskDesirer(
	logger lager.Logger,
	jobClient JobClient,
	secretsClient SecretsCreatorDeleter,
	defaultStagingNamespace string,
	tlsConfig []StagingConfigTLS,
	serviceAccountName string,
	stagingServiceAccountName string,
	registrySecretName string,
	rootfsVersion string,
) *TaskDesirer {
	return &TaskDesirer{
		logger:                    logger,
		jobClient:                 jobClient,
		secretsClient:             secretsClient,
		defaultStagingNamespace:   defaultStagingNamespace,
		tlsConfig:                 tlsConfig,
		serviceAccountName:        serviceAccountName,
		stagingServiceAccountName: stagingServiceAccountName,
		registrySecretName:        registrySecretName,
		rootfsVersion:             rootfsVersion,
	}
}

func NewTaskDesirerWithEiriniInstance(
	logger lager.Logger,
	jobClient JobClient,
	secretsClient SecretsCreatorDeleter,
	defaultStagingNamespace string,
	tlsConfig []StagingConfigTLS,
	serviceAccountName string,
	stagingServiceAccountName string,
	registrySecretName string,
	rootfsVersion string,
	eiriniInstance string,
) *TaskDesirer {
	desirer := NewTaskDesirer(
		logger,
		jobClient,
		secretsClient,
		defaultStagingNamespace,
		tlsConfig,
		serviceAccountName,
		stagingServiceAccountName,
		registrySecretName,
		rootfsVersion,
	)
	desirer.eiriniInstance = eiriniInstance
	return desirer
}

func (d *TaskDesirer) Desire(namespace string, task *opi.Task) error {
	job := d.toTaskJob(task)
	if imageInPrivateRegistry(task) {
		if err := d.addImagePullSecret(namespace, task, job); err != nil {
			return err
		}
	}

	_, err := d.jobClient.Create(namespace, job)
	return err
}

func (d *TaskDesirer) DesireStaging(task *opi.StagingTask) error {
	_, err := d.jobClient.Create(d.defaultStagingNamespace, d.toStagingJob(task))
	return err
}

func (d *TaskDesirer) Delete(guid string) (string, error) {
	return d.delete(guid, LabelGUID)
}

func (d *TaskDesirer) DeleteStaging(guid string) error {
	_, err := d.delete(guid, LabelStagingGUID)
	return err
}

func (d *TaskDesirer) delete(guid, label string) (string, error) {
	logger := d.logger.Session("delete", lager.Data{"guid": guid})
	jobs, err := d.jobClient.List(meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label, guid),
	})
	if err != nil {
		logger.Error("failed to list jobs", err)
		return "", err
	}
	if len(jobs.Items) != 1 {
		logger.Error("job with guid does not have 1 instance", nil, lager.Data{"instances": len(jobs.Items)})
		return "", fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs.Items))
	}

	job := jobs.Items[0]
	if err := d.deleteDockerRegistrySecret(job); err != nil {
		return "", err
	}

	callbackURL := job.Annotations[AnnotationCompletionCallback]
	backgroundPropagation := meta_v1.DeletePropagationBackground
	return callbackURL, d.jobClient.Delete(job.Namespace, job.Name, &meta_v1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
}

func (d *TaskDesirer) toTaskJob(task *opi.Task) *batch.Job {
	job := d.toJob(task)
	job.Spec.Template.Spec.ServiceAccountName = d.serviceAccountName
	job.Labels[LabelSourceType] = taskSourceType
	job.Labels[LabelName] = task.Name
	job.Labels[LabelEiriniInstance] = d.eiriniInstance
	job.Annotations[AnnotationCompletionCallback] = task.CompletionCallback
	job.Spec.Template.Annotations[AnnotationGUID] = task.GUID
	job.Spec.Template.Annotations[AnnotationOpiTaskContainerName] = opiTaskContainerName
	job.Spec.Template.Annotations[AnnotationCompletionCallback] = task.CompletionCallback

	envs := getEnvs(task)
	containers := []corev1.Container{
		{
			Name:            opiTaskContainerName,
			Image:           task.Image,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envs,
			Command:         task.Command,
		},
	}

	job.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{
			Name: d.registrySecretName,
		},
	}

	job.Spec.Template.Spec.Containers = containers

	return job
}

func (d *TaskDesirer) createTaskSecret(namespace string, task *opi.Task) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	secret.GenerateName = dockerImagePullSecretNamePrefix(task.AppName, task.SpaceName, task.GUID)
	secret.Type = corev1.SecretTypeDockerConfigJson

	dockerConfig := dockerutils.NewDockerConfig(
		task.PrivateRegistry.Server,
		task.PrivateRegistry.Username,
		task.PrivateRegistry.Password,
	)
	dockerConfigJSON, err := dockerConfig.JSON()
	if err != nil {
		return nil, err
	}
	secret.StringData = map[string]string{
		dockerutils.DockerConfigKey: dockerConfigJSON,
	}

	return d.secretsClient.Create(namespace, secret)
}

func (d *TaskDesirer) toStagingJob(task *opi.StagingTask) *batch.Job {
	job := d.toJob(task.Task)

	job.Spec.Template.Spec.ServiceAccountName = d.stagingServiceAccountName

	secretsVolume := corev1.Volume{
		Name: eirini.CertsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: d.getVolumeSources(),
			},
		},
	}

	secretsVolumeMount := corev1.VolumeMount{
		Name:      eirini.CertsVolumeName,
		ReadOnly:  true,
		MountPath: eirini.CertsMountPath,
	}

	outputVolume, outputVolumeMount := getVolume(eirini.RecipeOutputName, eirini.RecipeOutputLocation)
	buildpacksVolume, buildpacksVolumeMount := getVolume(eirini.RecipeBuildPacksName, eirini.RecipeBuildPacksDir)
	workspaceVolume, workspaceVolumeMount := getVolume(eirini.RecipeWorkspaceName, eirini.RecipeWorkspaceDir)
	buildpackCacheVolume, buildpackCacheVolumeMount := getVolume(eirini.BuildpackCacheName, eirini.BuildpackCacheDir)

	var downloaderVolumeMounts, executorVolumeMounts, uploaderVolumeMounts []corev1.VolumeMount

	downloaderVolumeMounts = append(downloaderVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount, buildpackCacheVolumeMount)
	executorVolumeMounts = append(executorVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount, outputVolumeMount, buildpackCacheVolumeMount)
	uploaderVolumeMounts = append(uploaderVolumeMounts, secretsVolumeMount, outputVolumeMount, buildpackCacheVolumeMount)

	envs := getEnvs(task.Task)
	initContainers := []corev1.Container{
		{
			Name:            "opi-task-downloader",
			Image:           task.DownloaderImage,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envs,
			VolumeMounts:    downloaderVolumeMounts,
		},
		{
			Name:            "opi-task-executor",
			Image:           task.ExecutorImage,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envs,
			VolumeMounts:    executorVolumeMounts,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory:           *resource.NewScaledQuantity(task.MemoryMB, resource.Mega),
					corev1.ResourceCPU:              toCPUMillicores(task.CPUWeight),
					corev1.ResourceEphemeralStorage: *resource.NewScaledQuantity(task.DiskMB, resource.Mega),
				},
			},
		},
	}

	containers := []corev1.Container{
		{
			Name:            "opi-task-uploader",
			Image:           task.UploaderImage,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envs,
			VolumeMounts:    uploaderVolumeMounts,
		},
	}

	job.Spec.Template.Spec.Containers = containers
	job.Spec.Template.Spec.InitContainers = initContainers

	volumes := []corev1.Volume{secretsVolume, outputVolume, buildpacksVolume, workspaceVolume, buildpackCacheVolume}
	job.Spec.Template.Spec.Volumes = volumes

	job.Annotations[AnnotationStagingGUID] = task.GUID

	job.Labels[LabelSourceType] = stagingSourceType
	job.Labels[LabelStagingGUID] = task.GUID
	job.Spec.Template.Labels[LabelStagingGUID] = task.GUID

	return job
}

func (d *TaskDesirer) getVolumeSources() []corev1.VolumeProjection {
	volumeSources := []corev1.VolumeProjection{}
	for _, conf := range d.tlsConfig {
		keyToPaths := []corev1.KeyToPath{}
		for _, keyPath := range conf.KeyPaths {
			keyToPaths = append(keyToPaths, corev1.KeyToPath{Key: keyPath.Key, Path: keyPath.Path})
		}
		volumeSources = append(volumeSources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: conf.SecretName,
				},
				Items: keyToPaths,
			},
		})
	}

	return volumeSources
}

func getEnvs(task *opi.Task) []corev1.EnvVar {
	envs := MapToEnvVar(task.Env)
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
		{Name: eirini.EnvCFInstanceAddr, Value: ""},
		{Name: eirini.EnvCFInstancePort, Value: ""},
		{Name: eirini.EnvCFInstancePorts, Value: "[]"},
	}

	envs = append(envs, fieldEnvs...)
	return envs
}

func getVolume(name, path string) (corev1.Volume, corev1.VolumeMount) {
	mount := corev1.VolumeMount{
		Name:      name,
		MountPath: path,
	}

	vol := corev1.Volume{
		Name: name,
	}

	return vol, mount
}

func (d *TaskDesirer) toJob(task *opi.Task) *batch.Job {
	automountServiceAccountToken := false
	runAsNonRoot := true

	job := &batch.Job{
		Spec: batch.JobSpec{
			Parallelism:  int32ptr(parallelism),
			Completions:  int32ptr(completions),
			BackoffLimit: int32ptr(0),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					RestartPolicy:                corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    int64ptr(VcapUID),
					},
				},
			},
		},
	}

	namePrefix := fmt.Sprintf("%s-%s", task.AppName, task.SpaceName)
	namePrefix = fmt.Sprintf("%s-", utils.SanitizeName(namePrefix, task.GUID))
	job.GenerateName = namePrefix

	job.Labels = map[string]string{
		LabelGUID:                        task.GUID,
		LabelAppGUID:                     task.AppGUID,
		rootfspatcher.RootfsVersionLabel: d.rootfsVersion,
	}

	job.Annotations = map[string]string{
		AnnotationAppName:              task.AppName,
		AnnotationAppID:                task.AppGUID,
		AnnotationOrgName:              task.OrgName,
		AnnotationOrgGUID:              task.OrgGUID,
		AnnotationSpaceName:            task.SpaceName,
		AnnotationSpaceGUID:            task.SpaceGUID,
		corev1.SeccompPodAnnotationKey: corev1.SeccompProfileRuntimeDefault,
	}

	job.Spec.Template.Labels = job.Labels
	job.Spec.Template.Annotations = job.Annotations

	return job
}

func (d *TaskDesirer) deleteDockerRegistrySecret(job batch.Job) error {
	dockerSecretNamePrefix := dockerImagePullSecretNamePrefix(
		job.Annotations[AnnotationAppName],
		job.Annotations[AnnotationSpaceName],
		job.Labels[LabelGUID],
	)

	for _, secret := range job.Spec.Template.Spec.ImagePullSecrets {
		if !strings.HasPrefix(secret.Name, dockerSecretNamePrefix) {
			continue
		}
		if err := d.secretsClient.Delete(job.Namespace, secret.Name); err != nil {
			return err
		}
	}

	return nil
}

func (d *TaskDesirer) addImagePullSecret(namespace string, task *opi.Task, job *batch.Job) error {
	createdSecret, err := d.createTaskSecret(namespace, task)
	if err != nil {
		return err
	}

	spec := &job.Spec.Template.Spec
	spec.ImagePullSecrets = append(spec.ImagePullSecrets, corev1.LocalObjectReference{
		Name: createdSecret.Name,
	})
	return nil
}

func dockerImagePullSecretNamePrefix(appName, spaceName, taskGUID string) string {
	secretNamePrefix := fmt.Sprintf("%s-%s", appName, spaceName)
	return fmt.Sprintf("%s-registry-secret-", utils.SanitizeName(secretNamePrefix, taskGUID))
}

func imageInPrivateRegistry(task *opi.Task) bool {
	return task.PrivateRegistry != nil && task.PrivateRegistry.Username != "" && task.PrivateRegistry.Password != ""
}
