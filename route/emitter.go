package route

import (
	"encoding/json"
	"fmt"

	nats "github.com/nats-io/go-nats"
)

const publisherSubject = "router.register"

type Publisher interface {
	Publish(subj string, data []byte) error
}

type NATSPublisher struct {
	NatsClient *nats.Conn
}

func (p *NATSPublisher) Publish(subj string, data []byte) error {
	return p.NatsClient.Publish(subj, data)
}

type Emitter struct {
	publisher Publisher
	scheduler TaskScheduler
	work      <-chan []RegistryMessage
}

func NewEmitter(publisher Publisher, workChannel chan []RegistryMessage, scheduler TaskScheduler) *Emitter {
	return &Emitter{
		publisher: publisher,
		scheduler: scheduler,
		work:      workChannel,
	}
}

func (r *Emitter) Start() {
	r.scheduler.Schedule(func() error {
		select {
		case batch := <-r.work:
			r.emit(batch)
		}
		return nil
	})
}

func (r *Emitter) emit(batch []RegistryMessage) {
	for _, route := range batch {
		routeJSON, err := json.Marshal(route)
		if err != nil {
			fmt.Println("Faild to marshal route message:", err.Error())
			continue
		}

		if err = r.publisher.Publish(publisherSubject, routeJSON); err != nil {
			fmt.Println("failed to publish route:", err.Error())
		}
	}
}
