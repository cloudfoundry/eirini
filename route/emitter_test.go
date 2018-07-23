package route_test

import (
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/eirini/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/route"
)

var _ = Describe("Emitter", func() {

	var (
		scheduler   *routefakes.FakeTaskScheduler
		publisher   *routefakes.FakePublisher
		workChannel chan []RegistryMessage
		emitter     *Emitter
		messages    []RegistryMessage
	)

	const (
		host     = "our.host.com"
		natsSubj = "router.register"
	)

	getExpectedData := func() [][]byte {
		expectedData := [][]byte{}
		for _, m := range messages {
			messageJSON, err := json.Marshal(m)
			Expect(err).ToNot(HaveOccurred())
			expectedData = append(expectedData, messageJSON)
		}

		return expectedData
	}

	getPublishData := func() [][]byte {
		publishData := [][]byte{}
		for i := 0; i < len(messages); i++ {
			subj, data := publisher.PublishArgsForCall(i)
			Expect(subj).To(Equal(natsSubj))

			publishData = append(publishData, data)
		}
		return publishData
	}

	BeforeEach(func() {
		scheduler = new(routefakes.FakeTaskScheduler)
		publisher = new(routefakes.FakePublisher)
		workChannel = make(chan []RegistryMessage, 1)

		messages = []RegistryMessage{
			RegistryMessage{
				Host: host,
				URIs: []string{"uri1", "uri2", "uri3"},
			},
			RegistryMessage{
				Host: host,
				URIs: []string{"uri4", "uri5", "uri6"},
			},
		}
	})

	AfterEach(func() {
		close(workChannel)
	})

	JustBeforeEach(func() {
		emitter = NewEmitter(publisher, workChannel, scheduler)
		emitter.Start()
	})

	assertInteractionsWithFakes := func() {
		It("should publish the routes", func() {
			task := scheduler.ScheduleArgsForCall(0)
			workChannel <- messages

			err := task()
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(time.Millisecond * 100) //TODO: Think of a better way (abstract goroutine? waitgroups?)

			Expect(publisher.PublishCallCount()).To(Equal(len(messages)))
			publishData := getPublishData()
			expectedData := getExpectedData()

			for _, e := range expectedData {
				Expect(publishData).To(ContainElement(e))
			}
		})
	}

	Context("When emitter is started", func() {
		assertInteractionsWithFakes()
	})

	Context("When the publisher returns an error", func() {
		BeforeEach(func() {
			publisher.PublishReturns(errors.New("Failed to publish message"))
		})

		assertInteractionsWithFakes()
	})
})
