package bifrost

import (
	"context"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
)

//counterfeiter:generate . TaskConverter

type TaskConverter interface {
	ConvertTask(taskGUID string, request cf.TaskRequest) (opi.Task, error)
}

//counterfeiter:generate . TaskDesirer

type TaskDesirer interface {
	Desire(task *opi.Task) error
	Delete(name string) error
}

type Task struct {
	Converter   TaskConverter
	TaskDesirer TaskDesirer
}

func (b *Task) TransferTask(ctx context.Context, taskGUID string, taskRequest cf.TaskRequest) error {
	desiredTask, err := b.Converter.ConvertTask(taskGUID, taskRequest)
	if err != nil {
		return errors.Wrap(err, "failed to convert task")
	}

	return errors.Wrap(b.TaskDesirer.Desire(&desiredTask), "failed to desire")
}

func (b *Task) CompleteTask(taskGUID string) error {
	return errors.Wrapf(b.TaskDesirer.Delete(taskGUID), "failed to delete task %s", taskGUID)
}
