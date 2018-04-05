package route

import (
	"encoding/json"
	"fmt"

	nats "github.com/nats-io/go-nats"
)

type RouteEmitter struct {
	NatsClient *nats.Conn
	Work       chan []RegistryMessage
}

func (r *RouteEmitter) Start() {
	for {
		select {
		case batch := <-r.Work:
			go r.emit(batch)
		}
	}
}

func (r *RouteEmitter) emit(batch []RegistryMessage) {
	for _, route := range batch {
		routeJson, err := json.Marshal(route)
		if err != nil {
			fmt.Println("Faild to marshal route message:", err.Error())
		}

		if err = r.NatsClient.Publish("router.register", routeJson); err != nil {
			fmt.Println("failed to publish route:", err.Error())
		}
	}
}
