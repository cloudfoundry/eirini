package jobs

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobGetter

type JobGetter interface {
	GetByGUID(ctx context.Context, guid string, includeCompleted bool) ([]batch.Job, error)
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

func (g *Getter) Get(ctx context.Context, taskGUID string) (*opi.Task, error) {
	jobs, err := g.jobGetter.GetByGUID(ctx, taskGUID, false)
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
