package k8s

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	taskSourceType       = "TASK"
	opiTaskContainerName = "opi-task"
	parallelism          = 1
	completions          = 1
)

//counterfeiter:generate . JobCreatingClient
//counterfeiter:generate . SecretsCreator

type JobCreatingClient interface {
	Create(namespace string, job *batch.Job) (*batch.Job, error)
	GetByGUID(guid string, includeCompleted bool) ([]batch.Job, error)
	List(includeCompleted bool) ([]batch.Job, error)
}

type SecretsCreator interface {
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
}

type KeyPath struct {
	Key  string
	Path string
}

type TaskDesirer struct {
	logger                            lager.Logger
	jobClient                         JobCreatingClient
	secretsCreator                    SecretsCreator
	serviceAccountName                string
	registrySecretName                string
	allowAutomountServiceAccountToken bool
}

func NewTaskDesirer(
	logger lager.Logger,
	jobClient JobCreatingClient,
	secretsCreator SecretsCreator,
	serviceAccountName string,
	registrySecretName string,
	allowAutomountServiceAccountToken bool,
) *TaskDesirer {
	return &TaskDesirer{
		logger:                            logger.Session("task-desirer"),
		jobClient:                         jobClient,
		secretsCreator:                    secretsCreator,
		serviceAccountName:                serviceAccountName,
		registrySecretName:                registrySecretName,
		allowAutomountServiceAccountToken: allowAutomountServiceAccountToken,
	}
}

func NewTaskDesirerWithEiriniInstance(
	logger lager.Logger,
	jobClient JobCreatingClient,
	secretsCreator SecretsCreator,
	serviceAccountName string,
	registrySecretName string,
	allowAutomountServiceAccountToken bool,
) *TaskDesirer {
	desirer := NewTaskDesirer(
		logger,
		jobClient,
		secretsCreator,
		serviceAccountName,
		registrySecretName,
		allowAutomountServiceAccountToken,
	)

	return desirer
}

func (d *TaskDesirer) Desire(namespace string, task *opi.Task, opts ...DesireOption) error {
	logger := d.logger.Session("desire", lager.Data{"guid": task.GUID, "name": task.Name, "namespace": namespace})

	job := d.toTaskJob(task)

	if imageInPrivateRegistry(task) {
		if err := d.addImagePullSecret(namespace, task, job); err != nil {
			logger.Error("failed-to-add-image-pull-secret", err)

			return err
		}
	}

	job.Namespace = namespace

	for _, opt := range opts {
		err := opt(job)
		if err != nil {
			logger.Error("failed-to-apply-option", err)

			return errors.Wrap(err, "failed to apply options")
		}
	}

	_, err := d.jobClient.Create(namespace, job)
	if err != nil {
		logger.Error("failed-to-create-job", err)

		return errors.Wrap(err, "failed to create job")
	}

	return nil
}

func (d *TaskDesirer) Get(taskGUID string) (*opi.Task, error) {
	jobs, err := d.jobClient.GetByGUID(taskGUID, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get job")
	}

	switch len(jobs) {
	case 0:
		return nil, eirini.ErrNotFound
	case 1:
		return toTask(jobs[0]), nil
	default:
		return nil, fmt.Errorf("multiple jobs found for task GUID %q", taskGUID)
	}
}

func (d *TaskDesirer) List() ([]*opi.Task, error) {
	jobs, err := d.jobClient.List(false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list jobs")
	}

	tasks := make([]*opi.Task, 0, len(jobs))
	for _, job := range jobs {
		tasks = append(tasks, toTask(job))
	}

	return tasks, nil
}

func (d *TaskDesirer) toTaskJob(task *opi.Task) *batch.Job {
	job := d.toJob(task)
	job.Spec.Template.Spec.ServiceAccountName = d.serviceAccountName
	job.Labels[LabelSourceType] = taskSourceType
	job.Labels[LabelName] = task.Name
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
		return nil, errors.Wrap(err, "failed-to-get-docker-config")
	}

	secret.StringData = map[string]string{
		dockerutils.DockerConfigKey: dockerConfigJSON,
	}

	return d.secretsCreator.Create(namespace, secret)
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

func (d *TaskDesirer) toJob(task *opi.Task) *batch.Job {
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

	if !d.allowAutomountServiceAccountToken {
		automountServiceAccountToken := false
		job.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	name := fmt.Sprintf("%s-%s", task.AppName, task.SpaceName)
	sanitizedName := utils.SanitizeName(name, task.GUID)

	if task.Name != "" {
		sanitizedName = fmt.Sprintf("%s-%s", sanitizedName, task.Name)
	}

	job.Name = utils.SanitizeNameWithMaxStringLen(sanitizedName, task.GUID, 50)

	job.Labels = map[string]string{
		LabelGUID:    task.GUID,
		LabelAppGUID: task.AppGUID,
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

func (d *TaskDesirer) addImagePullSecret(namespace string, task *opi.Task, job *batch.Job) error {
	createdSecret, err := d.createTaskSecret(namespace, task)
	if err != nil {
		return errors.Wrap(err, "failed to create task secret")
	}

	spec := &job.Spec.Template.Spec
	spec.ImagePullSecrets = append(spec.ImagePullSecrets, corev1.LocalObjectReference{
		Name: createdSecret.Name,
	})

	return nil
}

func imageInPrivateRegistry(task *opi.Task) bool {
	return task.PrivateRegistry != nil && task.PrivateRegistry.Username != "" && task.PrivateRegistry.Password != ""
}

func toTask(job batch.Job) *opi.Task {
	return &opi.Task{
		GUID: job.Labels[LabelGUID],
	}
}
