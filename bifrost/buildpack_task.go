package bifrost

import (
	"context"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
)

type BuildpackTask struct {
	Converter   Converter
	TaskDesirer opi.TaskDesirer
}

func (b *BuildpackTask) TransferTask(ctx context.Context, taskGUID string, taskRequest cf.TaskRequest) error {
	desiredTask, err := b.Converter.ConvertTask(taskGUID, taskRequest)
	if err != nil {
		return errors.Wrap(err, "failed to convert task")
	}

	return errors.Wrap(b.TaskDesirer.Desire(&desiredTask), "failed to desire")
}
