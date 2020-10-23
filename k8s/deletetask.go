package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobDeletingClient
//counterfeiter:generate . SecretsDeleter

type JobDeletingClient interface {
	GetByGUID(guid string, includeCompleted bool) ([]batchv1.Job, error)
	Delete(namespace string, name string) error
}

type SecretsDeleter interface {
	Delete(namespace, name string) error
}

type TaskDeleter struct {
	logger         lager.Logger
	jobClient      JobDeletingClient
	secretsDeleter SecretsDeleter
}

func NewTaskDeleter(
	logger lager.Logger,
	jobClient JobDeletingClient,
	secretsDeleter SecretsDeleter,
) *TaskDeleter {
	return &TaskDeleter{
		logger:         logger,
		jobClient:      jobClient,
		secretsDeleter: secretsDeleter,
	}
}

func (d *TaskDeleter) Delete(guid string) (string, error) {
	logger := d.logger.Session("delete", lager.Data{"guid": guid})

	job, err := d.getJobByGUID(logger, guid)
	if err != nil {
		return "", err
	}

	return d.delete(logger, job)
}

func (d *TaskDeleter) DeleteStaging(guid string) error {
	_, err := d.Delete(guid)

	return err
}

func (d *TaskDeleter) getJobByGUID(logger lager.Logger, guid string) (batchv1.Job, error) {
	jobs, err := d.jobClient.GetByGUID(guid, true)
	if err != nil {
		logger.Error("failed-to-list-jobs", err)

		return batchv1.Job{}, err
	}

	if len(jobs) != 1 {
		logger.Error("job-does-not-have-1-instance", nil, lager.Data{"instances": len(jobs)})

		return batchv1.Job{}, fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs))
	}

	return jobs[0], nil
}

func (d *TaskDeleter) delete(logger lager.Logger, job batchv1.Job) (string, error) {
	if err := d.deleteDockerRegistrySecret(logger, job); err != nil {
		return "", err
	}

	callbackURL := job.Annotations[AnnotationCompletionCallback]

	if len(job.OwnerReferences) != 0 {
		return callbackURL, nil
	}

	if err := d.jobClient.Delete(job.Namespace, job.Name); err != nil {
		logger.Error("failed-to-delete-job", err)

		return "", err
	}

	return callbackURL, nil
}

func (d *TaskDeleter) deleteDockerRegistrySecret(logger lager.Logger, job batchv1.Job) error {
	dockerSecretNamePrefix := dockerImagePullSecretNamePrefix(
		job.Annotations[AnnotationAppName],
		job.Annotations[AnnotationSpaceName],
		job.Labels[LabelGUID],
	)

	for _, secret := range job.Spec.Template.Spec.ImagePullSecrets {
		if !strings.HasPrefix(secret.Name, dockerSecretNamePrefix) {
			continue
		}

		if err := d.secretsDeleter.Delete(job.Namespace, secret.Name); err != nil {
			logger.Error("failed-to-delete-secret", err, lager.Data{"name": secret.Name, "namespace": job.Namespace})

			return errors.Wrap(err, "failed to delete secret")
		}
	}

	return nil
}

func dockerImagePullSecretNamePrefix(appName, spaceName, taskGUID string) string {
	secretNamePrefix := fmt.Sprintf("%s-%s", appName, spaceName)

	return fmt.Sprintf("%s-registry-secret-", utils.SanitizeName(secretNamePrefix, taskGUID))
}
