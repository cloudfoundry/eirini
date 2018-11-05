package route

type Message struct {
	Address            string
	Port               uint32
	TLSPort            uint32
	InstanceID         string
	Routes             []string
	UnregisteredRoutes []string
	Name               string
}

type Informer interface {
	Start(work chan<- *Message)
}
