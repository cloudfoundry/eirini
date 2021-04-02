package k8s

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/lager"
	batch "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobClient

type JobClient interface {
	Create(ctx context.Context, namespace string, job *batch.Job) (*batch.Job, error)
	List(ctx context.Context, includeCompleted bool) ([]batch.Job, error)
	GetByGUID(ctx context.Context, guid string, includeCompleted bool) ([]batch.Job, error)
	Delete(ctx context.Context, namespace string, name string) error
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
	secretsClient SecretsClient,
	taskToJobConverter jobs.TaskToJobConverter,
) *TaskClient {
	return &TaskClient{
		Desirer: jobs.NewDesirer(logger, taskToJobConverter, jobClient, secretsClient),
		Getter:  jobs.NewGetter(jobClient),
		Deleter: jobs.NewDeleter(logger, jobClient, jobClient),
		Lister:  jobs.NewLister(jobClient),
	}
}
