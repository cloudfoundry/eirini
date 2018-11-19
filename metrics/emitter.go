package metrics

import (
	"code.cloudfoundry.org/eirini/route"
)

type Emitter struct {
	scheduler route.TaskScheduler
	forwarder Forwarder
	work      <-chan []Message
}

type Message struct {
	AppID       string
	IndexID     string
	CPU         float64
	Memory      float64
	MemoryQuota float64
	Disk        float64
	DiskQuota   float64
}

//go:generate counterfeiter . Forwarder
type Forwarder interface {
	Forward(Message)
}

func NewEmitter(work <-chan []Message, scheduler route.TaskScheduler, forwarder Forwarder) *Emitter {
	return &Emitter{
		scheduler: scheduler,
		forwarder: forwarder,
		work:      work,
	}
}

func (e *Emitter) Start() {
	e.scheduler.Schedule(func() error {
		messages := <-e.work
		for _, m := range messages {
			e.forwarder.Forward(m)
		}
		return nil
	})
}
