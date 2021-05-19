package jobs

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobDeleter

type JobDeleter interface {
	Delete(ctx context.Context, namespace string, name string) error
}

type Deleter struct {
	logger     lager.Logger
	jobGetter  JobGetter
	jobDeleter JobDeleter
}

func NewDeleter(
	logger lager.Logger,
	jobGetter JobGetter,
	jobDeleter JobDeleter,
) Deleter {
	return Deleter{
		logger:     logger,
		jobGetter:  jobGetter,
		jobDeleter: jobDeleter,
	}
}

func (d *Deleter) Delete(ctx context.Context, guid string) (string, error) {
	logger := d.logger.Session("delete", lager.Data{"guid": guid})

	job, err := d.getJobByGUID(ctx, logger, guid)
	if err != nil {
		return "", err
	}

	return d.delete(ctx, logger, job)
}

func (d *Deleter) getJobByGUID(ctx context.Context, logger lager.Logger, guid string) (batchv1.Job, error) {
	jobs, err := d.jobGetter.GetByGUID(ctx, guid, true)
	if err != nil {
		logger.Error("failed-to-list-jobs", err)

		return batchv1.Job{}, errors.Wrap(err, "failed to list jobs")
	}

	if len(jobs) != 1 {
		logger.Error("job-does-not-have-1-instance", nil, lager.Data{"instances": len(jobs)})

		return batchv1.Job{}, fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs))
	}

	return jobs[0], nil
}

func (d *Deleter) delete(ctx context.Context, logger lager.Logger, job batchv1.Job) (string, error) {
	callbackURL := job.Annotations[AnnotationCompletionCallback]

	if err := d.jobDeleter.Delete(ctx, job.Namespace, job.Name); err != nil {
		logger.Error("failed-to-delete-job", err)

		return "", errors.Wrap(err, "failed to delete job")
	}

	return callbackURL, nil
}
