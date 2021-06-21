package jobs

import (
	"fmt"
	"strconv"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/utils"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	taskContainerName   = "opi-task"
	parallelism         = 1
	completions         = 1
	sanitizedNameMaxLen = 50
)

type Converter struct {
	serviceAccountName                string
	registrySecretName                string
	allowAutomountServiceAccountToken bool
	latestMigration                   int
}

func NewTaskToJobConverter(
	serviceAccountName string,
	registrySecretName string,
	allowAutomountServiceAccountToken bool,
	latestMigration int,
) *Converter {
	return &Converter{
		serviceAccountName:                serviceAccountName,
		registrySecretName:                registrySecretName,
		allowAutomountServiceAccountToken: allowAutomountServiceAccountToken,
		latestMigration:                   latestMigration,
	}
}

func (m *Converter) Convert(task *api.Task, privateRegistrySecret *corev1.Secret) *batch.Job {
	job := m.toJob(task)
	job.Spec.Template.Spec.ServiceAccountName = m.serviceAccountName
	job.Labels[LabelSourceType] = TaskSourceType
	job.Labels[LabelName] = task.Name
	job.Annotations[AnnotationCompletionCallback] = task.CompletionCallback
	job.Spec.Template.Annotations[AnnotationGUID] = task.GUID
	job.Spec.Template.Annotations[AnnotationTaskContainerName] = taskContainerName
	job.Spec.Template.Annotations[AnnotationCompletionCallback] = task.CompletionCallback

	envs := getEnvs(task)
	containers := []corev1.Container{
		{
			Name:            taskContainerName,
			Image:           task.Image,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envs,
			Command:         task.Command,
		},
	}

	job.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{
			Name: m.registrySecretName,
		},
	}

	if privateRegistrySecret != nil {
		job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets,
			corev1.LocalObjectReference{Name: privateRegistrySecret.Name})
	}

	job.Spec.Template.Spec.Containers = containers

	return job
}

func (m *Converter) toJob(task *api.Task) *batch.Job {
	runAsNonRoot := true

	job := &batch.Job{
		Spec: batch.JobSpec{
			Parallelism:  int32ptr(parallelism),
			Completions:  int32ptr(completions),
			BackoffLimit: int32ptr(0),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
				},
			},
		},
	}

	if !m.allowAutomountServiceAccountToken {
		automountServiceAccountToken := false
		job.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	name := fmt.Sprintf("%s-%s", task.AppName, task.SpaceName)
	sanitizedName := utils.SanitizeName(name, task.GUID)

	if task.Name != "" {
		sanitizedName = fmt.Sprintf("%s-%s", sanitizedName, task.Name)
	}

	job.Name = utils.SanitizeNameWithMaxStringLen(sanitizedName, task.GUID, sanitizedNameMaxLen)

	job.Labels = map[string]string{
		LabelGUID:    task.GUID,
		LabelAppGUID: task.AppGUID,
	}

	job.Annotations = map[string]string{
		AnnotationAppName:                task.AppName,
		AnnotationAppID:                  task.AppGUID,
		AnnotationOrgName:                task.OrgName,
		AnnotationOrgGUID:                task.OrgGUID,
		AnnotationSpaceName:              task.SpaceName,
		AnnotationSpaceGUID:              task.SpaceGUID,
		corev1.SeccompPodAnnotationKey:   corev1.SeccompProfileRuntimeDefault,
		shared.AnnotationLatestMigration: strconv.Itoa(m.latestMigration),
	}

	job.Spec.Template.Labels = job.Labels
	job.Spec.Template.Annotations = job.Annotations

	return job
}

func getEnvs(task *api.Task) []corev1.EnvVar {
	envs := shared.MapToEnvVar(task.Env)
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

func int32ptr(i int) *int32 {
	u := int32(i)

	return &u
}
