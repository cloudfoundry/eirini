package route

import (
	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
)

type CollectorScheduler struct {
	Collector Collector
	Scheduler util.TaskScheduler
}

func (c CollectorScheduler) Start(work chan<- *Message) {
	c.Scheduler.Schedule(func() error {
		routes, err := c.Collector.Collect()
		if err != nil {
			return errors.Wrap(err, "failed to collect routes")
		}
		for _, r := range routes {
			r := r
			work <- &r
		}
		return nil
	})
}
