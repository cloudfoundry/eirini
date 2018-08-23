package route

import (
	"code.cloudfoundry.org/eirini"
)

const (
	httpPort = 80
	tlsPort  = 443
)

type Collector struct {
	RouteLister  RouteLister
	Scheduler    TaskScheduler
	Work         chan<- []*eirini.Routes
	KubeEndpoint string
}

//go:generate counterfeiter . RouteLister
type RouteLister interface {
	ListRoutes() ([]*eirini.Routes, error)
}

func (r *Collector) Start() {
	r.Scheduler.Schedule(r.collectRoutes)
}

func (r *Collector) collectRoutes() error {
	routes, err := r.RouteLister.ListRoutes()
	if err != nil {
		return err
	}
	r.Work <- routes
	return nil
}
