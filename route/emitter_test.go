package route_test

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/julz/cube/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/julz/cube/route"
)

var _ = Describe("Emitter", func() {

	Context("RouteEmitter", func() {
		var (
			scheduler   *routefakes.FakeTaskScheduler
			publisher   *routefakes.FakePublisher
			workChannel chan []RegistryMessage
			emitter     RouteEmitter
			messages    []RegistryMessage
		)

		const (
			host     = "our.host.com"
			natsSubj = "router.register"
		)

		getExpectedData := func() [][]byte {
			expectedData := [][]byte{}
			for _, m := range messages {
				messageJson, err := json.Marshal(m)
				Expect(err).ToNot(HaveOccurred())
				expectedData = append(expectedData, messageJson)
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
			emitter = RouteEmitter{
				Publisher: publisher,
				Scheduler: scheduler,
				Work:      workChannel,
			}

			emitter.Start()
		})

		assertInteractionsWithFakes := func() {
			It("should use the scheduler", func() {
				Expect(scheduler.ScheduleCallCount()).To(Equal(1))
			})

			It("should publish the routes", func() {
				task := scheduler.ScheduleArgsForCall(0)
				workChannel <- messages
				task()
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

		Context("When the publisher throws an error", func() {
			BeforeEach(func() {
				publisher.PublishReturns(errors.New("Failed to publish message"))
			})

			assertInteractionsWithFakes()
		})
	})
})
