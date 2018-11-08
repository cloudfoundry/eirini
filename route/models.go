package route

import "errors"

type Message struct {
	Address            string
	Port               uint32
	TLSPort            uint32
	InstanceID         string
	Routes             []string
	UnregisteredRoutes []string
	Name               string
}

func NewMessage(name, instanceID, address string, port uint32) (*Message, error) {
	if len(address) == 0 {
		return nil, errors.New("missing address")
	}

	return &Message{
		Name:       name,
		InstanceID: instanceID,
		Address:    address,
		Port:       port,
		TLSPort:    0,
	}, nil
}

type Informer interface {
	Start(work chan<- *Message)
}
