package k8s

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
)

const (
	ActiveDeadlineSeconds = 900
	stagingSourceType     = "STG"
	parallelism           = 1
	completions           = 1
)

//go:generate counterfeiter . JobClient
type JobClient interface {
	Create(*batch.Job) (*batch.Job, error)
	Delete(name string, deleteOpts *meta_v1.DeleteOptions) error
}

type TaskDesirer struct {
	Namespace       string
	CertsSecretName string
	JobClient       JobClient
	Logger          lager.Logger
}

func (d *TaskDesirer) Desire(task *opi.Task) error {
	job := toJob(task)

	envs := getEnvs(task)
	containers := []v1.Container{
		{
			Name:            "opi-task",
			Image:           task.Image,
			ImagePullPolicy: v1.PullAlways,
			Env:             envs,
		},
	}

	job.Spec.Template.Spec.Containers = containers

	_, err := d.JobClient.Create(job)
	return err
}

func (d *TaskDesirer) DesireStaging(task *opi.StagingTask) error {
	job := d.toStagingJob(task)
	_, err := d.JobClient.Create(job)
	return err
}

func (d *TaskDesirer) Delete(name string) error {
	backgroundPropagation := meta_v1.DeletePropagationBackground
	err := d.JobClient.Delete(name, &meta_v1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
	return err
}

func (d *TaskDesirer) toStagingJob(task *opi.StagingTask) *batch.Job {
	job := toJob(task.Task)

	secretsVolume := v1.Volume{
		Name: eirini.CertsVolumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: d.CertsSecretName,
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

	var downloaderVolumeMounts, executorVolumeMounts, uploaderVolumeMounts []v1.VolumeMount

	downloaderVolumeMounts = append(downloaderVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount)
	executorVolumeMounts = append(executorVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount, outputVolumeMount)
	uploaderVolumeMounts = append(uploaderVolumeMounts, secretsVolumeMount, outputVolumeMount)

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

	volumes := []v1.Volume{secretsVolume, outputVolume, buildpacksVolume, workspaceVolume}
	job.Spec.Template.Spec.Volumes = volumes

	job.Annotations[AnnotationStagingGUID] = task.StagingGUID

	job.Labels[LabelSourceType] = stagingSourceType
	job.Labels[LabelStagingGUID] = task.StagingGUID
	job.Spec.Template.Labels[LabelStagingGUID] = task.StagingGUID

	return job
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

func toJob(task *opi.Task) *batch.Job {
	automountServiceAccountToken := false
	runAsNonRoot := true

	job := &batch.Job{
		Spec: batch.JobSpec{
			ActiveDeadlineSeconds: int64ptr(ActiveDeadlineSeconds),
			Parallelism:           int32ptr(parallelism),
			Completions:           int32ptr(completions),
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

	job.Name = task.Env[eirini.EnvStagingGUID]

	job.Labels = map[string]string{
		LabelAppGUID: task.AppGUID,
		LabelGUID:    task.Env[eirini.EnvAppID],
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

	return job
}
