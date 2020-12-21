package jobs

import (
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . TaskToJob
//counterfeiter:generate . JobCreator
//counterfeiter:generate . SecretCreator

type TaskToJob func(*opi.Task) *batch.Job

type JobCreator interface {
	Create(namespace string, job *batch.Job) (*batch.Job, error)
}

type SecretCreator interface {
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
}

type Desirer struct {
	logger        lager.Logger
	taskToJob     TaskToJob
	jobCreator    JobCreator
	secretCreator SecretCreator
}

func NewDesirer(
	logger lager.Logger,
	taskToJob TaskToJob,
	jobCreator JobCreator,
	secretCreator SecretCreator,
) Desirer {
	return Desirer{
		logger:        logger,
		taskToJob:     taskToJob,
		jobCreator:    jobCreator,
		secretCreator: secretCreator,
	}
}

func (d *Desirer) Desire(namespace string, task *opi.Task, opts ...shared.Option) error {
	logger := d.logger.Session("desire-task", lager.Data{"guid": task.GUID, "name": task.Name, "namespace": namespace})

	job := d.taskToJob(task)

	if imageInPrivateRegistry(task) {
		if err := d.addImagePullSecret(namespace, task, job); err != nil {
			logger.Error("failed-to-add-image-pull-secret", err)

			return err
		}
	}

	job.Namespace = namespace

	if err := shared.ApplyOpts(job, opts...); err != nil {
		logger.Error("failed-to-apply-option", err)

		return err
	}

	_, err := d.jobCreator.Create(namespace, job)
	if err != nil {
		logger.Error("failed-to-create-job", err)

		return errors.Wrap(err, "failed to create job")
	}

	return nil
}

func imageInPrivateRegistry(task *opi.Task) bool {
	return task.PrivateRegistry != nil && task.PrivateRegistry.Username != "" && task.PrivateRegistry.Password != ""
}

func (d *Desirer) addImagePullSecret(namespace string, task *opi.Task, job *batch.Job) error {
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

func (d *Desirer) createTaskSecret(namespace string, task *opi.Task) (*corev1.Secret, error) {
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

	return d.secretCreator.Create(namespace, secret)
}

func dockerImagePullSecretNamePrefix(appName, spaceName, taskGUID string) string {
	secretNamePrefix := fmt.Sprintf("%s-%s", appName, spaceName)

	return fmt.Sprintf("%s-registry-secret-", utils.SanitizeName(secretNamePrefix, taskGUID))
}
