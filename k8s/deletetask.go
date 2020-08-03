package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//counterfeiter:generate . JobListerDeleter
//counterfeiter:generate . SecretsDeleter

type JobListerDeleter interface {
	List(opts meta_v1.ListOptions) (*batch.JobList, error)
	Delete(namespace string, name string, options meta_v1.DeleteOptions) error
}

type SecretsDeleter interface {
	Delete(namespace, name string) error
}

type TaskDeleter struct {
	logger         lager.Logger
	jobClient      JobListerDeleter
	secretsDeleter SecretsDeleter
	eiriniInstance string
}

func NewTaskDeleter(
	logger lager.Logger,
	jobClient JobListerDeleter,
	secretsDeleter SecretsDeleter,
	eiriniInstance string,
) *TaskDeleter {
	return &TaskDeleter{
		logger:         logger,
		jobClient:      jobClient,
		secretsDeleter: secretsDeleter,
		eiriniInstance: eiriniInstance,
	}
}

func (d *TaskDeleter) Delete(guid string) (string, error) {
	logger := d.logger.Session("delete", lager.Data{"guid": guid})

	return d.delete(logger, guid, LabelGUID)
}

func (d *TaskDeleter) DeleteStaging(guid string) error {
	logger := d.logger.Session("delete-staging", lager.Data{"guid": guid})
	_, err := d.delete(logger, guid, LabelStagingGUID)

	return err
}

func (d *TaskDeleter) delete(logger lager.Logger, guid, label string) (string, error) {
	jobs, err := d.jobClient.List(meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
			label, guid,
			LabelEiriniInstance, d.eiriniInstance,
		),
	})
	if err != nil {
		logger.Error("failed-to-list-jobs", err)

		return "", err
	}

	if len(jobs.Items) != 1 {
		logger.Error("job-does-not-have-1-instance", nil, lager.Data{"instances": len(jobs.Items)})

		return "", fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs.Items))
	}

	job := jobs.Items[0]
	if err = d.deleteDockerRegistrySecret(logger, job); err != nil {
		return "", err
	}

	callbackURL := job.Annotations[AnnotationCompletionCallback]

	if len(job.OwnerReferences) != 0 {
		return callbackURL, nil
	}

	backgroundPropagation := meta_v1.DeletePropagationBackground
	err = d.jobClient.Delete(job.Namespace, job.Name, meta_v1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})

	if err != nil {
		logger.Error("failed-to-delete-job", err)

		return "", err
	}

	return callbackURL, nil
}

func (d *TaskDeleter) deleteDockerRegistrySecret(logger lager.Logger, job batch.Job) error {
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
