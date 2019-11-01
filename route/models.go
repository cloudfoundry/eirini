package route

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

//go:generate counterfeiter . Collector
type Collector interface {
	Collect() ([]Message, error)
}
