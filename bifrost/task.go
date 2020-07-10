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
	Delete(guid string) (string, error)
}

//counterfeiter:generate . JSONClient

type JSONClient interface {
	Post(url string, data interface{}) error
}

type Task struct {
	Converter   TaskConverter
	TaskDesirer TaskDesirer
	JSONClient  JSONClient
}

func (b *Task) TransferTask(ctx context.Context, taskGUID string, taskRequest cf.TaskRequest) error {
	desiredTask, err := b.Converter.ConvertTask(taskGUID, taskRequest)
	if err != nil {
		return errors.Wrap(err, "failed to convert task")
	}

	return errors.Wrap(b.TaskDesirer.Desire(&desiredTask), "failed to desire")
}

func (b *Task) CancelTask(taskGUID string) error {
	callbackURL, err := b.TaskDesirer.Delete(taskGUID)
	if err != nil {
		return errors.Wrapf(err, "failed to delete task %s", taskGUID)
	}

	if len(callbackURL) == 0 {
		return nil
	}

	go func() {
		_ = b.JSONClient.Post(callbackURL, cf.TaskCompletedRequest{
			TaskGUID:      taskGUID,
			Failed:        true,
			FailureReason: "task was cancelled",
		})
	}()

	return nil
}
