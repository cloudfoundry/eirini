package metrics

import (
	"code.cloudfoundry.org/eirini/route"
)

type Emitter struct {
	scheduler route.TaskScheduler
	forwarder Forwarder
	work      <-chan Message
}

type Message struct {
	AppID       string
	IndexID     string
	CPU         float64
	Memory      float64
	MemoryQuota float64
	MemoryUnit  string
	Disk        float64
	DiskQuota   float64
	DiskUnit    string
}

//go:generate counterfeiter . Forwarder
type Forwarder interface {
	Forward(Message)
}

func NewEmitter(work <-chan Message, scheduler route.TaskScheduler, forwarder Forwarder) *Emitter {
	return &Emitter{
		scheduler: scheduler,
		forwarder: forwarder,
		work:      work,
	}
}

func (e *Emitter) Start() {
	e.scheduler.Schedule(func() error {
		message := <-e.work
		e.forwarder.Forward(message)
		return nil
	})
}
