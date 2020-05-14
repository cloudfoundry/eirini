package k8s

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
)

const (
	stagingSourceType = "STG"
	taskSourceType    = "TASK"
	parallelism       = 1
	completions       = 1
)

//counterfeiter:generate . JobClient
type JobClient interface {
	Create(*batch.Job) (*batch.Job, error)
	Delete(guid string, deleteOpts *meta_v1.DeleteOptions) error
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
	Namespace          string
	CertsSecretName    string
	TLSConfig          []StagingConfigTLS
	ServiceAccountName string
	RegistrySecretName string
	JobClient          JobClient
	Logger             lager.Logger
	SecretsClient      SecretsClient
}

func (d *TaskDesirer) Desire(task *opi.Task) error {
	taskJob, err := d.toTaskJob(task)
	if err != nil {
		return err
	}

	_, err = d.JobClient.Create(taskJob)
	return err
}

func (d *TaskDesirer) DesireStaging(task *opi.StagingTask) error {
	_, err := d.JobClient.Create(d.toStagingJob(task))
	return err
}

func (d *TaskDesirer) Delete(guid string) error {
	return d.delete(guid, LabelGUID)
}

func (d *TaskDesirer) DeleteStaging(guid string) error {
	return d.delete(guid, LabelStagingGUID)
}

func (d *TaskDesirer) delete(guid, label string) error {
	logger := d.Logger.Session("delete", lager.Data{"guid": guid})
	jobs, err := d.JobClient.List(meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label, guid),
	})
	if err != nil {
		logger.Error("failed to list jobs", err)
		return err
	}
	if len(jobs.Items) != 1 {
		logger.Error("job with guid does not have 1 instance", nil, lager.Data{"instances": len(jobs.Items)})
		return fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs.Items))
	}

	backgroundPropagation := meta_v1.DeletePropagationBackground
	return d.JobClient.Delete(jobs.Items[0].Name, &meta_v1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
}

func (d *TaskDesirer) toTaskJob(task *opi.Task) (*batch.Job, error) {
	job := d.toJob(task)
	job.Labels[LabelSourceType] = taskSourceType
	job.Labels[LabelName] = task.Name

	envs := getEnvs(task)
	containers := []v1.Container{
		{
			Name:            "opi-task",
			Image:           task.Image,
			ImagePullPolicy: v1.PullAlways,
			Env:             envs,
			Command:         task.Command,
		},
	}

	job.Spec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
		{
			Name: d.RegistrySecretName,
		},
	}

	if task.PrivateRegistry != nil && task.PrivateRegistry.Username != "" && task.PrivateRegistry.Password != "" {
		createdSecret, err := d.createTaskSecret(task)
		if err != nil {
			return nil, err
		}

		job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, v1.LocalObjectReference{
			Name: createdSecret.Name,
		})
	}

	job.Spec.Template.Spec.Containers = containers

	return job, nil
}

func (d *TaskDesirer) createTaskSecret(task *opi.Task) (*v1.Secret, error) {
	secret := &v1.Secret{}

	secretNamePrefix := fmt.Sprintf("%s-%s", task.AppName, task.SpaceName)
	secretNamePrefix = fmt.Sprintf("%s-registry-secret-", utils.SanitizeName(secretNamePrefix, task.GUID))
	secret.GenerateName = secretNamePrefix

	secret.Type = v1.SecretTypeDockerConfigJson

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

	return d.SecretsClient.Create(secret)
}

func (d *TaskDesirer) toStagingJob(task *opi.StagingTask) *batch.Job {
	job := d.toJob(task.Task)

	secretsVolume := v1.Volume{
		Name: eirini.CertsVolumeName,
		VolumeSource: v1.VolumeSource{
			Projected: &v1.ProjectedVolumeSource{
				Sources: d.getVolumeSources(),
			},
		},
	}

	secretsVolumeMount := v1.VolumeMount{
		Name:      eirini.CertsVolumeName,
		ReadOnly:  true,
		MountPath: eirini.CertsMountPath,
	}

	outputVolume, outputVolumeMount := getVolume(eirini.RecipeOutputName, eirini.RecipeOutputLocation)
	buildpacksVolume, buildpacksVolumeMount := getVolume(eirini.RecipeBuildPacksName, eirini.RecipeBuildPacksDir)
	workspaceVolume, workspaceVolumeMount := getVolume(eirini.RecipeWorkspaceName, eirini.RecipeWorkspaceDir)
	buildpackCacheVolume, buildpackCacheVolumeMount := getVolume(eirini.BuildpackCacheName, eirini.BuildpackCacheDir)

	var downloaderVolumeMounts, executorVolumeMounts, uploaderVolumeMounts []v1.VolumeMount

	downloaderVolumeMounts = append(downloaderVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount, buildpackCacheVolumeMount)
	executorVolumeMounts = append(executorVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount, outputVolumeMount, buildpackCacheVolumeMount)
	uploaderVolumeMounts = append(uploaderVolumeMounts, secretsVolumeMount, outputVolumeMount, buildpackCacheVolumeMount)

	envs := getEnvs(task.Task)
	initContainers := []v1.Container{
		{
			Name:            "opi-task-downloader",
			Image:           task.DownloaderImage,
			ImagePullPolicy: v1.PullAlways,
			Env:             envs,
			VolumeMounts:    downloaderVolumeMounts,
		},
		{
			Name:            "opi-task-executor",
			Image:           task.ExecutorImage,
			ImagePullPolicy: v1.PullAlways,
			Env:             envs,
			VolumeMounts:    executorVolumeMounts,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory:           *resource.NewScaledQuantity(task.MemoryMB, resource.Mega),
					v1.ResourceCPU:              toCPUMillicores(task.CPUWeight),
					v1.ResourceEphemeralStorage: *resource.NewScaledQuantity(task.DiskMB, resource.Mega),
				},
			},
		},
	}

	containers := []v1.Container{
		{
			Name:            "opi-task-uploader",
			Image:           task.UploaderImage,
			ImagePullPolicy: v1.PullAlways,
			Env:             envs,
			VolumeMounts:    uploaderVolumeMounts,
		},
	}

	job.Spec.Template.Spec.Containers = containers
	job.Spec.Template.Spec.InitContainers = initContainers

	volumes := []v1.Volume{secretsVolume, outputVolume, buildpacksVolume, workspaceVolume, buildpackCacheVolume}
	job.Spec.Template.Spec.Volumes = volumes

	job.Annotations[AnnotationStagingGUID] = task.GUID

	job.Labels[LabelSourceType] = stagingSourceType
	job.Labels[LabelStagingGUID] = task.GUID
	job.Spec.Template.Labels[LabelStagingGUID] = task.GUID

	return job
}

func (d *TaskDesirer) getVolumeSources() []v1.VolumeProjection {
	volumeSources := []v1.VolumeProjection{}
	for _, conf := range d.TLSConfig {
		keyToPaths := []v1.KeyToPath{}
		for _, keyPath := range conf.KeyPaths {
			keyToPaths = append(keyToPaths, v1.KeyToPath{Key: keyPath.Key, Path: keyPath.Path})
		}
		volumeSources = append(volumeSources, v1.VolumeProjection{
			Secret: &v1.SecretProjection{
				LocalObjectReference: v1.LocalObjectReference{
					Name: conf.SecretName,
				},
				Items: keyToPaths,
			},
		})
	}

	return volumeSources
}

func getEnvs(task *opi.Task) []v1.EnvVar {
	envs := MapToEnvVar(task.Env)
	fieldEnvs := []v1.EnvVar{
		{
			Name: eirini.EnvPodName,
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: eirini.EnvCFInstanceIP,
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: eirini.EnvCFInstanceInternalIP,
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
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

func getVolume(name, path string) (v1.Volume, v1.VolumeMount) {
	mount := v1.VolumeMount{
		Name:      name,
		MountPath: path,
	}

	vol := v1.Volume{
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
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					RestartPolicy:                v1.RestartPolicyNever,
					SecurityContext: &v1.PodSecurityContext{
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
		LabelGUID:    task.GUID,
		LabelAppGUID: task.AppGUID,
	}

	job.Annotations = map[string]string{
		AnnotationAppName:   task.AppName,
		AnnotationAppID:     task.AppGUID,
		AnnotationOrgName:   task.OrgName,
		AnnotationOrgGUID:   task.OrgGUID,
		AnnotationSpaceName: task.SpaceName,
		AnnotationSpaceGUID: task.SpaceGUID,
	}

	job.Spec.Template.Labels = job.Labels
	job.Spec.Template.Annotations = job.Annotations
	job.Spec.Template.Spec.ServiceAccountName = d.ServiceAccountName

	return job
}
