package k8s

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ActiveDeadlineSeconds = 900
	stagingSourceType     = "STG"
	parallelism           = 1
	completions           = 1
)

type TaskDesirer struct {
	Namespace       string
	CCUploaderIP    string
	CertsSecretName string
	Client          kubernetes.Interface
	Logger          lager.Logger
}

func (d *TaskDesirer) Desire(task *opi.Task) error {
	job := toJob(task)

	containers := []v1.Container{
		{
			Name:            "opi-task",
			Image:           task.Image,
			ImagePullPolicy: v1.PullAlways,
			Env:             MapToEnvVar(task.Env),
		},
	}

	job.Spec.Template.Spec.Containers = containers

	_, err := d.Client.BatchV1().Jobs(d.Namespace).Create(job)
	return errors.Wrap(err, "job already exists")
}

func (d *TaskDesirer) DesireStaging(task *opi.StagingTask) error {
	job := d.toStagingJob(task)
	_, err := d.Client.BatchV1().Jobs(d.Namespace).Create(job)
	return errors.Wrap(err, "job already exists")
}

func (d *TaskDesirer) Delete(name string) error {
	backgroundPropagation := meta_v1.DeletePropagationBackground
	err := d.Client.BatchV1().Jobs(d.Namespace).Delete(name, &meta_v1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
	return errors.Wrap(err, "job does not exist")
}

func (d *TaskDesirer) toStagingJob(task *opi.StagingTask) *batch.Job {
	job := toJob(task.Task)

	job.Spec.Template.Spec.HostAliases = []v1.HostAlias{
		{
			IP:        d.CCUploaderIP,
			Hostnames: []string{eirini.CCUploaderInternalURL},
		},
	}

	secretsVolume := v1.Volume{
		Name: eirini.CCCertsVolumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: d.CertsSecretName,
				Items: []v1.KeyToPath{
					{Key: eirini.CCAPICertName, Path: eirini.CCAPICertName},
					{Key: eirini.CCAPIKeyName, Path: eirini.CCAPIKeyName},
					{Key: eirini.CCUploaderCertName, Path: eirini.CCUploaderCertName},
					{Key: eirini.CCUploaderKeyName, Path: eirini.CCUploaderKeyName},
					{Key: eirini.CCInternalCACertName, Path: eirini.CCInternalCACertName},
				},
			},
		},
	}

	secretsVolumeMount := v1.VolumeMount{
		Name:      eirini.CCCertsVolumeName,
		ReadOnly:  true,
		MountPath: eirini.CCCertsMountPath,
	}

	outputVolume, outputVolumeMount := getVolume(eirini.RecipeOutputName, eirini.RecipeOutputLocation)
	buildpacksVolume, buildpacksVolumeMount := getVolume(eirini.RecipeBuildPacksName, eirini.RecipeBuildPacksDir)
	workspaceVolume, workspaceVolumeMount := getVolume(eirini.RecipeWorkspaceName, eirini.RecipeWorkspaceDir)

	var downloaderVolumeMounts, executorVolumeMounts, uploaderVolumeMounts []v1.VolumeMount

	downloaderVolumeMounts = append(downloaderVolumeMounts, secretsVolumeMount, buildpacksVolumeMount, workspaceVolumeMount)
	executorVolumeMounts = append(executorVolumeMounts, buildpacksVolumeMount, workspaceVolumeMount, outputVolumeMount)
	uploaderVolumeMounts = append(uploaderVolumeMounts, secretsVolumeMount, outputVolumeMount)

	initContainers := []v1.Container{
		{
			Name:            "opi-task-downloader",
			Image:           task.DownloaderImage,
			ImagePullPolicy: v1.PullAlways,
			Env:             MapToEnvVar(task.Env),
			VolumeMounts:    downloaderVolumeMounts,
		},
		{
			Name:            "opi-task-executor",
			Image:           task.ExecutorImage,
			ImagePullPolicy: v1.PullAlways,
			Env:             MapToEnvVar(task.Env),
			VolumeMounts:    executorVolumeMounts,
		},
	}

	containers := []v1.Container{
		{
			Name:            "opi-task-uploader",
			Image:           task.UploaderImage,
			ImagePullPolicy: v1.PullAlways,
			Env:             MapToEnvVar(task.Env),
			VolumeMounts:    uploaderVolumeMounts,
		},
	}

	job.Spec.Template.Spec.Containers = containers
	job.Spec.Template.Spec.InitContainers = initContainers

	volumes := []v1.Volume{secretsVolume, outputVolume, buildpacksVolume, workspaceVolume}
	job.Spec.Template.Spec.Volumes = volumes

	return job
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
	job := &batch.Job{
		Spec: batch.JobSpec{
			ActiveDeadlineSeconds: int64ptr(ActiveDeadlineSeconds),
			Parallelism:           int32ptr(parallelism),
			Completions:           int32ptr(completions),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					RestartPolicy:                v1.RestartPolicyNever,
				},
			},
		},
	}

	job.Name = task.Env[eirini.EnvStagingGUID]

	labels := map[string]string{
		"guid":        task.Env[eirini.EnvAppID],
		"source_type": stagingSourceType,
	}

	job.Spec.Template.Labels = labels
	job.Labels = labels

	return job
}
