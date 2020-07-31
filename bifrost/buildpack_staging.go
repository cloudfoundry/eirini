package bifrost

import (
	"context"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

//counterfeiter:generate . StagingConverter
//counterfeiter:generate . StagingDesirer
//counterfeiter:generate . StagingDeleter
//counterfeiter:generate . StagingCompleter

type StagingConverter interface {
	ConvertStaging(stagingGUID string, request cf.StagingRequest) (opi.StagingTask, error)
}

type StagingDesirer interface {
	DesireStaging(task *opi.StagingTask) error
}

type StagingDeleter interface {
	DeleteStaging(name string) error
}

type StagingCompleter interface {
	CompleteStaging(cf.StagingCompletedRequest) error
}

type BuildpackStaging struct {
	Converter        StagingConverter
	StagingDesirer   StagingDesirer
	StagingDeleter   StagingDeleter
	StagingCompleter StagingCompleter
	Logger           lager.Logger
}

func (b *BuildpackStaging) TransferStaging(ctx context.Context, stagingGUID string, stagingRequest cf.StagingRequest) error {
	desiredStaging, err := b.Converter.ConvertStaging(stagingGUID, stagingRequest)
	if err != nil {
		return errors.Wrap(err, "failed to convert staging task")
	}

	return errors.Wrap(b.StagingDesirer.DesireStaging(&desiredStaging), "failed to desire")
}

func (b *BuildpackStaging) CompleteStaging(taskCompletedRequest cf.StagingCompletedRequest) error {
	l := b.Logger.Session("complete-staging", lager.Data{"task-guid": taskCompletedRequest.TaskGUID})
	l.Debug("Complete staging")

	return multierr.Combine(
		b.StagingCompleter.CompleteStaging(taskCompletedRequest),
		b.StagingDeleter.DeleteStaging(taskCompletedRequest.TaskGUID),
	)
}
