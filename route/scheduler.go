package route

import (
	"context"

	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
)

//counterfeiter:generate . Emitter

type Emitter interface {
	Emit(ctx context.Context, msg Message)
}

type CollectorScheduler struct {
	Collector Collector
	Scheduler util.TaskScheduler
	Emitter   Emitter
}

func (c CollectorScheduler) Start() {
	c.Scheduler.Schedule(func() error {
		ctx := context.Background()
		routes, err := c.Collector.Collect(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to collect routes")
		}
		for _, r := range routes {
			c.Emitter.Emit(ctx, r)
		}

		return nil
	})
}
