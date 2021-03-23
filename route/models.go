package route

import "context"

//counterfeiter:generate . Collector

type Routes struct {
	RegisteredRoutes   []string
	UnregisteredRoutes []string
}

type Message struct {
	Routes
	Address    string
	Port       uint32
	TLSPort    uint32
	InstanceID string
	Name       string
}

type Informer interface {
	Start()
}

type Collector interface {
	Collect(ctx context.Context) ([]Message, error)
}
