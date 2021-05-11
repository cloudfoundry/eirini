package jobs

import (
	"context"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobGetter

type JobGetter interface {
	GetByGUID(ctx context.Context, guid string, includeCompleted bool) ([]batchv1.Job, error)
}

type Getter struct {
	jobGetter JobGetter
}

func NewGetter(
	jobGetter JobGetter,
) Getter {
	return Getter{
		jobGetter: jobGetter,
	}
}

func (g *Getter) Get(ctx context.Context, taskGUID string) (*api.Task, error) {
	jobs, err := g.jobGetter.GetByGUID(ctx, taskGUID, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get job")
	}

	job, err := getSingleJob(jobs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get task with GUID %q", taskGUID)
	}

	return toTask(job), nil
}

func getSingleJob(jobs []batchv1.Job) (batchv1.Job, error) {
	switch len(jobs) {
	case 0:
		return batchv1.Job{}, eirini.ErrNotFound
	case 1:
		return jobs[0], nil
	default:
		return batchv1.Job{}, errors.New("multiple jobs found for task")
	}
}
