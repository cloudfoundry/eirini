package k8s

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/lager"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . JobClient
//counterfeiter:generate . SecretClient

type JobClient interface {
	Create(ctx context.Context, namespace string, job *batch.Job) (*batch.Job, error)
	List(ctx context.Context, includeCompleted bool) ([]batch.Job, error)
	GetByGUID(ctx context.Context, guid string, includeCompleted bool) ([]batch.Job, error)
	Delete(ctx context.Context, namespace string, name string) error
}

type SecretClient interface {
	Create(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	Delete(ctx context.Context, namespace, name string) error
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
	taskToJobConverter jobs.TaskToJobConverter,
) *TaskClient {
	return &TaskClient{
		Desirer: jobs.NewDesirer(logger, taskToJobConverter, jobClient, secretClient),
		Getter:  jobs.NewGetter(jobClient),
		Deleter: jobs.NewDeleter(logger, jobClient, jobClient, secretClient),
		Lister:  jobs.NewLister(jobClient),
	}
}
