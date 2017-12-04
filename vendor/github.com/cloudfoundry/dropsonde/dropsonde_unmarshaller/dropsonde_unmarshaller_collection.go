package dropsonde_unmarshaller

import (
	"sync"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

// A DropsondeUnmarshallerCollection is a collection of DropsondeUnmarshaller instances.
type DropsondeUnmarshallerCollection struct {
	unmarshallers []*DropsondeUnmarshaller
	logger        *gosteno.Logger
}

// NewDropsondeUnmarshallerCollection instantiates a DropsondeUnmarshallerCollection,
// creates the specified number of DropsondeUnmarshaller instances and logs to the
// provided logger.
func NewDropsondeUnmarshallerCollection(logger *gosteno.Logger, size int) *DropsondeUnmarshallerCollection {
	var unmarshallers []*DropsondeUnmarshaller
	for i := 0; i < size; i++ {
		unmarshallers = append(unmarshallers, NewDropsondeUnmarshaller(logger))
	}

	logger.Debugf("dropsondeUnmarshallerCollection: created %v unmarshallers", size)

	return &DropsondeUnmarshallerCollection{
		logger:        logger,
		unmarshallers: unmarshallers,
	}
}

// Returns the number of unmarshallers in its collection.
func (u *DropsondeUnmarshallerCollection) Size() int {
	return len(u.unmarshallers)
}

// Run calls Run on each marshaller in its collection.
// This is done in separate go routines.
func (u *DropsondeUnmarshallerCollection) Run(inputChan <-chan []byte, outputChan chan<- *events.Envelope, waitGroup *sync.WaitGroup) {
	for _, unmarshaller := range u.unmarshallers {
		go func(um *DropsondeUnmarshaller) {
			defer waitGroup.Done()
			um.Run(inputChan, outputChan)
		}(unmarshaller)
	}
}
