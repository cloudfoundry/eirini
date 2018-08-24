package route

import (
	"code.cloudfoundry.org/eirini"
)

type Collector struct {
	RouteLister Lister
	Scheduler   TaskScheduler
	Work        chan<- []*eirini.Routes
}

//go:generate counterfeiter . Lister
type Lister interface {
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
