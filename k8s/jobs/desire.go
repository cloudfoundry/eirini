package jobs

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//counterfeiter:generate . TaskToJobConverter
//counterfeiter:generate . JobCreator
//counterfeiter:generate . SecretsClient

type TaskToJobConverter interface {
	Convert(*opi.Task, *corev1.Secret) *batch.Job
}

type JobCreator interface {
	Create(ctx context.Context, namespace string, job *batch.Job) (*batch.Job, error)
}

type SecretsClient interface {
	Create(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	SetOwner(ctx context.Context, secret *corev1.Secret, owner metav1.Object) (*corev1.Secret, error)
	Delete(ctx context.Context, namespace string, name string) error
}

type Desirer struct {
	logger             lager.Logger
	taskToJobConverter TaskToJobConverter
	jobCreator         JobCreator
	secrets            SecretsClient
}

func NewDesirer(
	logger lager.Logger,
	taskToJobConverter TaskToJobConverter,
	jobCreator JobCreator,
	secretCreator SecretsClient,
) Desirer {
	return Desirer{
		logger:             logger,
		taskToJobConverter: taskToJobConverter,
		jobCreator:         jobCreator,
		secrets:            secretCreator,
	}
}

func (d *Desirer) Desire(ctx context.Context, namespace string, task *opi.Task, opts ...shared.Option) error {
	logger := d.logger.Session("desire-task", lager.Data{"guid": task.GUID, "name": task.Name, "namespace": namespace})

	var (
		err                   error
		privateRegistrySecret *corev1.Secret
	)

	if imageInPrivateRegistry(task) {
		privateRegistrySecret, err = d.createPrivateRegistrySecret(ctx, namespace, task)
		if err != nil {
			return errors.Wrap(err, "failed to create task secret")
		}
	}

	job := d.taskToJobConverter.Convert(task, privateRegistrySecret)

	job.Namespace = namespace

	if err = shared.ApplyOpts(job, opts...); err != nil {
		logger.Error("failed-to-apply-option", err)

		return err
	}

	job, err = d.jobCreator.Create(ctx, namespace, job)
	if err != nil {
		logger.Error("failed-to-create-job", err)

		return d.cleanupAndError(ctx, err, privateRegistrySecret)
	}

	if privateRegistrySecret != nil {
		_, err = d.secrets.SetOwner(ctx, privateRegistrySecret, job)
		if err != nil {
			return errors.Wrap(err, "failed-to-set-secret-ownership")
		}
	}

	return nil
}

func imageInPrivateRegistry(task *opi.Task) bool {
	return task.PrivateRegistry != nil && task.PrivateRegistry.Username != "" && task.PrivateRegistry.Password != ""
}

func (d *Desirer) createPrivateRegistrySecret(ctx context.Context, namespace string, task *opi.Task) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	secret.GenerateName = PrivateRegistrySecretGenerateName
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

	return d.secrets.Create(ctx, namespace, secret)
}

func (d *Desirer) cleanupAndError(ctx context.Context, jobCreationError error, privateRegistrySecret *corev1.Secret) error {
	resultError := multierror.Append(nil, jobCreationError)

	if privateRegistrySecret != nil {
		err := d.secrets.Delete(ctx, privateRegistrySecret.Namespace, privateRegistrySecret.Name)
		if err != nil {
			resultError = multierror.Append(resultError, errors.Wrap(err, "failed to cleanup registry secret"))
		}
	}

	return resultError
}
