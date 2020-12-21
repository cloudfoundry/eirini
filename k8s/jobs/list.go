package jobs

import (
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobLister

type JobLister interface {
	List(includeCompleted bool) ([]batch.Job, error)
}

type Lister struct {
	jobLister JobLister
}

func NewLister(
	jobLister JobLister,
) Lister {
	return Lister{
		jobLister: jobLister,
	}
}

func (l *Lister) List() ([]*opi.Task, error) {
	jobs, err := l.jobLister.List(false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list jobs")
	}

	tasks := make([]*opi.Task, 0, len(jobs))
	for _, job := range jobs {
		tasks = append(tasks, toTask(job))
	}

	return tasks, nil
}
