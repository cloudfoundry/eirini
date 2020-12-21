package k8s

import (
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/lager"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . JobClient
//counterfeiter:generate . SecretClient

type JobClient interface {
	Create(namespace string, job *batch.Job) (*batch.Job, error)
	List(includeCompleted bool) ([]batch.Job, error)
	GetByGUID(guid string, includeCompleted bool) ([]batch.Job, error)
	Delete(namespace string, name string) error
}

type SecretClient interface {
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	Delete(namespace, name string) error
}

type TaskClient struct {
	jobs.Desirer
	jobs.Getter
	jobs.Deleter
	jobs.Lister
}

func NewTaskClient(
	logger lager.Logger,
	jobClient JobClient,
	secretClient SecretClient,
	taskToJob jobs.TaskToJob,
) *TaskClient {
	return &TaskClient{
		Desirer: jobs.NewDesirer(logger, taskToJob, jobClient, secretClient),
		Getter:  jobs.NewGetter(jobClient),
		Deleter: jobs.NewDeleter(logger, jobClient, jobClient, secretClient),
		Lister:  jobs.NewLister(jobClient),
	}
}
