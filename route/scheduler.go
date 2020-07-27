package route

import (
	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
)

//counterfeiter:generate . Emitter

type Emitter interface {
	Emit(Message)
}

type CollectorScheduler struct {
	Collector Collector
	Scheduler util.TaskScheduler
	Emitter   Emitter
}

func (c CollectorScheduler) Start() {
	c.Scheduler.Schedule(func() error {
		routes, err := c.Collector.Collect()
		if err != nil {
			return errors.Wrap(err, "failed to collect routes")
		}
		for _, r := range routes {
			c.Emitter.Emit(r)
		}
		return nil
	})
}
