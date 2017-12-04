package events_test

import (
	"encoding/base64"
	"errors"
	"io"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vito/go-sse/sse"
)

var _ = Describe("EventSource", func() {
	var eventSource events.EventSource
	var fakeRawEventSource *eventfakes.FakeRawEventSource

	BeforeEach(func() {
		fakeRawEventSource = new(eventfakes.FakeRawEventSource)
		eventSource = events.NewEventSource(fakeRawEventSource)
	})

	Describe("Next", func() {
		Describe("Desired LRP events", func() {
			var desiredLRP *models.DesiredLRP

			BeforeEach(func() {
				desiredLRP = &models.DesiredLRP{
					ProcessGuid: "some-guid",
					Domain:      "some-domain",
					RootFs:      "some-rootfs",
					Action: models.WrapAction(&models.RunAction{
						Path: "true",
						User: "theuser",
					}),
				}
			})

			Context("when receiving a DesiredLRPCreatedEvent", func() {
				var expectedEvent *models.DesiredLRPCreatedEvent

				BeforeEach(func() {
					expectedEvent = models.NewDesiredLRPCreatedEvent(desiredLRP)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "hi",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					desiredLRPCreateEvent, ok := event.(*models.DesiredLRPCreatedEvent)
					Expect(ok).To(BeTrue())
					Expect(desiredLRPCreateEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a DesiredLRPChangedEvent", func() {
				var expectedEvent *models.DesiredLRPChangedEvent

				BeforeEach(func() {
					expectedEvent = models.NewDesiredLRPChangedEvent(desiredLRP, desiredLRP)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "hi",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					desiredLRPChangeEvent, ok := event.(*models.DesiredLRPChangedEvent)
					Expect(ok).To(BeTrue())
					Expect(desiredLRPChangeEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a DesiredLRPRemovedEvent", func() {
				var expectedEvent *models.DesiredLRPRemovedEvent

				BeforeEach(func() {
					expectedEvent = models.NewDesiredLRPRemovedEvent(desiredLRP)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					desiredLRPRemovedEvent, ok := event.(*models.DesiredLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(desiredLRPRemovedEvent).To(Equal(expectedEvent))
				})
			})
		})

		Describe("Actual LRP Events", func() {
			var actualLRPGroup *models.ActualLRPGroup
			var actualLRP *models.ActualLRP

			BeforeEach(func() {
				actualLRP = models.NewUnclaimedActualLRP(models.NewActualLRPKey("some-guid", 0, "some-domain"), 1)
				actualLRPGroup = models.NewRunningActualLRPGroup(actualLRP)
			})

			Context("when receiving a ActualLRPCreatedEvent", func() {
				var expectedEvent *models.ActualLRPCreatedEvent

				BeforeEach(func() {
					expectedEvent = models.NewActualLRPCreatedEvent(actualLRPGroup)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					actualLRPCreatedEvent, ok := event.(*models.ActualLRPCreatedEvent)
					Expect(ok).To(BeTrue())
					Expect(actualLRPCreatedEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a ActualLRPChangedEvent", func() {
				var expectedEvent *models.ActualLRPChangedEvent

				BeforeEach(func() {
					expectedEvent = models.NewActualLRPChangedEvent(actualLRPGroup, actualLRPGroup)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					actualLRPChangedEvent, ok := event.(*models.ActualLRPChangedEvent)
					Expect(ok).To(BeTrue())
					Expect(actualLRPChangedEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a ActualLRPRemovedEvent", func() {
				var expectedEvent *models.ActualLRPRemovedEvent

				BeforeEach(func() {
					expectedEvent = models.NewActualLRPRemovedEvent(actualLRPGroup)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					actualLRPRemovedEvent, ok := event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(actualLRPRemovedEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a ActualLRPCrashedEvent", func() {
				var expectedEvent *models.ActualLRPCrashedEvent

				BeforeEach(func() {
					expectedEvent = models.NewActualLRPCrashedEvent(actualLRP, actualLRP)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					actualLRPCrashedEvent, ok := event.(*models.ActualLRPCrashedEvent)
					Expect(ok).To(BeTrue())
					Expect(actualLRPCrashedEvent).To(Equal(expectedEvent))
				})
			})
		})

		Describe("Task events", func() {
			var task *models.Task

			BeforeEach(func() {
				task = model_helpers.NewValidTask("some-guid")
			})

			Context("when receiving a TaskCreatedEvent", func() {
				var expectedEvent *models.TaskCreatedEvent

				BeforeEach(func() {
					expectedEvent = models.NewTaskCreatedEvent(task)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					taskCreatedEvent, ok := event.(*models.TaskCreatedEvent)
					Expect(ok).To(BeTrue())
					Expect(taskCreatedEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a TaskChangedEvent", func() {
				var expectedEvent *models.TaskChangedEvent

				BeforeEach(func() {
					expectedEvent = models.NewTaskChangedEvent(task, task)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					taskChangedEvent, ok := event.(*models.TaskChangedEvent)
					Expect(ok).To(BeTrue())
					Expect(taskChangedEvent).To(Equal(expectedEvent))
				})
			})

			Context("when receiving a TaskRemovedEvent", func() {
				var expectedEvent *models.TaskRemovedEvent

				BeforeEach(func() {
					expectedEvent = models.NewTaskRemovedEvent(task)
					payload, err := proto.Marshal(expectedEvent)
					Expect(err).NotTo(HaveOccurred())
					payload = []byte(base64.StdEncoding.EncodeToString(payload))

					fakeRawEventSource.NextReturns(
						sse.Event{
							ID:   "sup",
							Name: string(expectedEvent.EventType()),
							Data: payload,
						},
						nil,
					)
				})

				It("returns the event", func() {
					event, err := eventSource.Next()
					Expect(err).NotTo(HaveOccurred())

					taskRemovedEvent, ok := event.(*models.TaskRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(taskRemovedEvent).To(Equal(expectedEvent))
				})
			})
		})

		Context("when receiving an unrecognized event", func() {
			BeforeEach(func() {
				payload := []byte(base64.StdEncoding.EncodeToString([]byte("garbage")))
				fakeRawEventSource.NextReturns(
					sse.Event{
						ID:   "sup",
						Name: "unrecognized-event-type",
						Data: payload,
					},
					nil,
				)
			})

			It("returns an unrecognized event error", func() {
				_, err := eventSource.Next()
				Expect(err).To(Equal(events.ErrUnrecognizedEventType))
			})
		})

		Context("when receiving a bad payload", func() {
			BeforeEach(func() {
				fakeRawEventSource.NextReturns(
					sse.Event{
						ID:   "sup",
						Name: models.EventTypeDesiredLRPCreated,
						Data: []byte{1, 1, 1, 1, 0, 255, 0, 1, 1, 1},
					},
					nil,
				)
			})

			It("returns a proto error", func() {
				_, err := eventSource.Next()
				Expect(err).To(BeAssignableToTypeOf(events.NewInvalidPayloadError(models.EventTypeDesiredLRPCreated, errors.New("whatever"))))
			})
		})

		Context("when receiving a bad payload that is base64 encoded", func() {
			BeforeEach(func() {
				encodedPayload := base64.StdEncoding.EncodeToString([]byte("garbage"))
				fakeRawEventSource.NextReturns(
					sse.Event{
						ID:   "sup",
						Name: models.EventTypeDesiredLRPCreated,
						Data: []byte(encodedPayload),
					},
					nil,
				)
			})

			It("returns a proto error", func() {
				_, err := eventSource.Next()
				Expect(err).To(BeAssignableToTypeOf(events.NewInvalidPayloadError(models.EventTypeDesiredLRPCreated, errors.New("whatever"))))
			})
		})

		Context("when receiving an empty payload", func() {
			BeforeEach(func() {
				fakeRawEventSource.NextReturns(
					sse.Event{
						ID:   "sup",
						Name: models.EventTypeDesiredLRPCreated,
						Data: []byte{},
					},
					nil,
				)
			})

			It("returns a proto error", func() {
				_, err := eventSource.Next()
				Expect(err).To(BeAssignableToTypeOf(events.NewInvalidPayloadError(models.EventTypeDesiredLRPCreated, errors.New("whatever"))))
			})

			It("includes a useful error message", func() {
				_, err := eventSource.Next()
				Expect(err.Error()).To(ContainSubstring("event with no data"))
			})
		})

		Context("when the raw event source returns an error", func() {
			var rawError error

			BeforeEach(func() {
				rawError = errors.New("raw-error")
				fakeRawEventSource.NextReturns(sse.Event{}, rawError)
			})

			It("propagates the error", func() {
				_, err := eventSource.Next()
				Expect(err).To(Equal(events.NewRawEventSourceError(rawError)))
			})
		})

		Context("when the raw event source returns io.EOF", func() {
			BeforeEach(func() {
				fakeRawEventSource.NextReturns(sse.Event{}, io.EOF)
			})

			It("returns io.EOF", func() {
				_, err := eventSource.Next()
				Expect(err).To(Equal(io.EOF))
			})
		})

		Context("when the raw event source returns sse.ErrSourceClosed", func() {
			BeforeEach(func() {
				fakeRawEventSource.NextReturns(sse.Event{}, sse.ErrSourceClosed)
			})

			It("returns models.ErrSourceClosed", func() {
				_, err := eventSource.Next()
				Expect(err).To(Equal(events.ErrSourceClosed))
			})
		})
	})

	Describe("Close", func() {
		Context("when the raw source closes normally", func() {
			It("closes the raw event source", func() {
				eventSource.Close()
				Expect(fakeRawEventSource.CloseCallCount()).To(Equal(1))
			})

			It("does not error", func() {
				err := eventSource.Close()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the raw source closes with error", func() {
			var rawError error

			BeforeEach(func() {
				rawError = errors.New("ka-boom")
				fakeRawEventSource.CloseReturns(rawError)
			})

			It("closes the raw event source", func() {
				eventSource.Close()
				Expect(fakeRawEventSource.CloseCallCount()).To(Equal(1))
			})

			It("propagates the error", func() {
				err := eventSource.Close()
				Expect(err).To(Equal(events.NewCloseError(rawError)))
			})
		})
	})
})
