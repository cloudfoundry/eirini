package dropsonde_unmarshaller_test

import (
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"

	"sync"

	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeUnmarshallerCollection", func() {
	var collection *dropsonde_unmarshaller.DropsondeUnmarshallerCollection

	BeforeEach(func() {
		collection = dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(loggertesthelper.Logger(), 5)
		metrics.Initialize(nil, nil)
	})

	Context("DropsondeUnmarshallerCollection", func() {
		It("creates the right number of unmarshallers", func() {
			Expect(collection.Size()).To(Equal(5))
		})
	})

	Context("Run", func() {
		var (
			inputChan  chan []byte
			outputChan chan *events.Envelope
			waitGroup  *sync.WaitGroup
		)

		BeforeEach(func() {
			inputChan = make(chan []byte)
			outputChan = make(chan *events.Envelope)
			waitGroup = &sync.WaitGroup{}
		})

		AfterEach(func() {
			close(inputChan)
			for i := 0; i < collection.Size(); i++ {
				<-outputChan
			}
			waitGroup.Wait()
		})

		It("doesn't block while there are unmarshallers idle", func() {
			waitGroup.Add(collection.Size())
			collection.Run(inputChan, outputChan, waitGroup)
			env := &events.Envelope{
				Origin:    proto.String("foo"),
				EventType: events.Envelope_LogMessage.Enum(),
			}
			bytes, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())
			done := make(chan struct{})
			go func() {
				defer close(done)
				for i := 0; i < 5; i++ {
					inputChan <- bytes
				}
			}()
			Eventually(done).Should(BeClosed())
			done = make(chan struct{})
			go func() {
				defer close(done)
				inputChan <- bytes
			}()
			Consistently(done).ShouldNot(BeClosed())

			Eventually(outputChan).Should(Receive())
			Eventually(done).Should(BeClosed())
		})
	})
})
