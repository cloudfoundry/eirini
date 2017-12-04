package handlers_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Event Handlers", func() {
	var (
		logger  lager.Logger
		handler handlers.EventController
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
	})

	var ItRecoversFromLostConnections = func(hubRef *events.Hub) {
		Describe("When the client connection is lost", func() {
			var (
				hub             events.Hub
				response        *http.Response
				server          *httptest.Server
				err             error
				eventStreamDone chan struct{}
			)

			BeforeEach(func() {
				eventStreamDone = make(chan struct{})
				hub = *hubRef
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Subscribe_r0(logger, w, r)
					close(eventStreamDone)
				}))
				response, err = http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				server.Close()
			})

			It("returns early", func() {
				reader := sse.NewReadCloser(response.Body)
				err := reader.Close()
				Expect(err).NotTo(HaveOccurred())
				go func() {
					for {
						hub.Emit(eventfakes.FakeEvent{Token: "A"})
					}
				}()
				Eventually(eventStreamDone, 10).Should(BeClosed())
			})
		})
	}

	var ItStreamsEventsFromHub = func(hubRef *events.Hub) {
		Describe("Streaming Events", func() {
			var (
				hub      events.Hub
				response *http.Response
				server   *httptest.Server
				err      error
			)

			BeforeEach(func() {
				hub = *hubRef
			})

			JustBeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Subscribe_r0(logger, w, r)
				}))
				response, err = http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				server.Close()
			})

			Context("when failing to subscribe to the event hub", func() {
				BeforeEach(func() {
					hub.Close()
				})

				It("returns an internal server error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when successfully subscribing to the event hub", func() {
				It("emits events from the hub to the connection", func() {
					reader := sse.NewReadCloser(response.Body)

					hub.Emit(&eventfakes.FakeEvent{Token: "A"})
					encodedPayload := base64.StdEncoding.EncodeToString([]byte("A"))

					Expect(reader.Next()).To(Equal(sse.Event{
						ID:   "0",
						Name: "fake",
						Data: []byte(encodedPayload),
					}))

					hub.Emit(&eventfakes.FakeEvent{Token: "B"})

					encodedPayload = base64.StdEncoding.EncodeToString([]byte("B"))
					Expect(reader.Next()).To(Equal(sse.Event{
						ID:   "1",
						Name: "fake",
						Data: []byte(encodedPayload),
					}))
				})

				It("returns Content-Type as text/event-stream", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("text/event-stream; charset=utf-8"))
					Expect(response.Header.Get("Cache-Control")).To(Equal("no-cache, no-store, must-revalidate"))
				})

				Context("when the source provides an unmarshalable event", func() {
					It("closes the event stream to the client", func() {
						hub.Emit(eventfakes.UnmarshalableEvent{Fn: func() {}})

						reader := sse.NewReadCloser(response.Body)
						_, err := reader.Next()
						Expect(err).To(Equal(io.EOF))
					})
				})

				Context("when the event source returns an error", func() {
					JustBeforeEach(func() {
						hub.Close()
					})

					It("closes the client event stream", func() {
						reader := sse.NewReadCloser(response.Body)
						_, err := reader.Next()
						Expect(err).To(Equal(io.EOF))
					})
				})
			})
		})
	}

	Describe("Subscribe_r0", func() {
		var (
			desiredHub events.Hub
			actualHub  events.Hub
		)

		BeforeEach(func() {
			desiredHub = events.NewHub()
			actualHub = events.NewHub()
			handler = handlers.NewEventHandler(desiredHub, actualHub)
		})

		AfterEach(func() {
			desiredHub.Close()
			actualHub.Close()
		})

		Describe("Subscribe to Desired Events", func() {

			ItStreamsEventsFromHub(&desiredHub)
			ItRecoversFromLostConnections(&desiredHub)

			It("migrates desired lrps down to v0", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Subscribe_r0(logger, w, r)
				}))

				response, err := http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())
				reader := sse.NewReadCloser(response.Body)

				desiredLRP := model_helpers.NewValidDesiredLRP("guid")
				event := models.NewDesiredLRPCreatedEvent(desiredLRP)

				migratedLRP := desiredLRP.VersionDownTo(format.V0)
				Expect(migratedLRP).NotTo(Equal(desiredLRP))
				migratedEvent := models.NewDesiredLRPCreatedEvent(migratedLRP)

				desiredHub.Emit(event)

				events := events.NewEventSource(reader)
				actualEvent, err := events.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(actualEvent).To(Equal(migratedEvent))

				server.Close()
			})
		})

		Describe("Subscribe to Actual Events", func() {
			Context("when cell id not specified", func() {
				ItStreamsEventsFromHub(&actualHub)
				ItRecoversFromLostConnections(&actualHub)
			})

			Context("when cell id is specified", func() {
				var (
					reader      *sse.ReadCloser
					requestBody interface{}
					cellId      = "cell-id"
					eventSource events.EventSource
					eventsCh    chan models.Event
					server      *httptest.Server
				)

				BeforeEach(func() {
					requestBody = nil
				})

				AfterEach(func() {
					server.Close()
				})

				JustBeforeEach(func() {
					By("creating server")
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						request := newTestRequest(requestBody)
						handler.Subscribe_r0(logger, w, request)
					}))

					By("starting server")
					response, err := http.Get(server.URL)
					Expect(err).NotTo(HaveOccurred())
					reader = sse.NewReadCloser(response.Body)

					eventSource = events.NewEventSource(reader)

					eventsCh = streamEvents(eventSource)
				})

				Context("ActualLRPChangedEvent", func() {
					var (
						expectedActualLRPBeforeEvent *models.ActualLRPChangedEvent
						expectedActualLRPAfterEvent  *models.ActualLRPChangedEvent
					)

					BeforeEach(func() {
						actualLRP := models.NewUnclaimedActualLRP(models.NewActualLRPKey("guid", 0, "some-domain"), 1)
						actualLRPGroupBefore := models.NewRunningActualLRPGroup(actualLRP)

						actualLRP = models.NewClaimedActualLRP(
							models.NewActualLRPKey("some-guid", 0, "some-domain"),
							models.NewActualLRPInstanceKey("instance-guid-1", "cell-id"),
							1,
						)
						actualLRPGroupAfter := models.NewRunningActualLRPGroup(actualLRP)
						expectedActualLRPBeforeEvent = models.NewActualLRPChangedEvent(actualLRPGroupBefore, actualLRPGroupAfter)

						actualLRP = models.NewUnclaimedActualLRP(models.NewActualLRPKey("some-guid", 0, "some-domain"), 1)
						unclaimedActualLRPGroupAgain := models.NewRunningActualLRPGroup(actualLRP)
						expectedActualLRPAfterEvent = models.NewActualLRPChangedEvent(actualLRPGroupAfter, unclaimedActualLRPGroupAgain)
					})

					JustBeforeEach(func() {
						By("sending actual lrp changed event")
						actualHub.Emit(expectedActualLRPBeforeEvent)
						actualHub.Emit(expectedActualLRPAfterEvent)
					})

					Context("subscriber with the right filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: cellId,
							}
						})

						Context("and an LRP transitions to evacuating", func() {
							BeforeEach(func() {
								actualLRP := models.NewClaimedActualLRP(
									models.NewActualLRPKey("some-guid", 0, "some-domain"),
									models.NewActualLRPInstanceKey("instance-guid-0", "cell-id"),
									1,
								)
								actualLRPGroupBefore := models.NewRunningActualLRPGroup(actualLRP)

								actualLRP = models.NewClaimedActualLRP(
									models.NewActualLRPKey("some-guid", 0, "some-domain"),
									models.NewActualLRPInstanceKey("instance-guid-1", "cell-id"),
									1,
								)
								actualLRPGroupAfter := models.NewEvacuatingActualLRPGroup(actualLRP)

								expectedActualLRPBeforeEvent = models.NewActualLRPChangedEvent(actualLRPGroupBefore, actualLRPGroupAfter)
							})

							It("receives changed events if the lrp is running on the cell", func() {
								Eventually(eventsCh).Should(Receive(Equal(expectedActualLRPBeforeEvent)))
							})
						})

						Context("and an evacuating LRP leaves the cell", func() {
							BeforeEach(func() {
								actualLRP := models.NewClaimedActualLRP(
									models.NewActualLRPKey("some-guid", 0, "some-domain"),
									models.NewActualLRPInstanceKey("instance-guid-0", "cell-id"),
									1,
								)
								actualLRPGroupBefore := models.NewEvacuatingActualLRPGroup(actualLRP)

								actualLRP = models.NewClaimedActualLRP(
									models.NewActualLRPKey("some-guid", 0, "some-domain"),
									models.NewActualLRPInstanceKey("instance-guid-1", "another-cell-id"),
									1,
								)
								actualLRPGroupAfter := models.NewRunningActualLRPGroup(actualLRP)
								expectedActualLRPBeforeEvent = models.NewActualLRPChangedEvent(actualLRPGroupBefore, actualLRPGroupAfter)
							})

							It("receives changed events if the lrp used to run on the cell", func() {
								Eventually(eventsCh).Should(Receive(Equal(expectedActualLRPBeforeEvent)))
							})
						})

						It("receives changed events if the lrp used to run on the cell", func() {
							Eventually(eventsCh).Should(Receive(Equal(expectedActualLRPBeforeEvent)))
						})

						It("receives changed events if the lrp started running on the cell", func() {
							Eventually(eventsCh).Should(Receive(Equal(expectedActualLRPAfterEvent)))
						})
					})

					Context("subscriber with the wrong filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: "another-cell-id",
							}
						})

						It("does not receive changed events if the lrp did not use to run on the cell", func() {
							Consistently(eventsCh).ShouldNot(Receive(Equal(expectedActualLRPBeforeEvent)))
						})

						It("does not receive changed events if the lrp did not start running on the cell", func() {
							Eventually(eventsCh).ShouldNot(Receive(Equal(expectedActualLRPAfterEvent)))
						})
					})
				})

				Context("ActualLRPCreatedEvent", func() {
					var (
						expectedEvent *models.ActualLRPCreatedEvent
					)

					BeforeEach(func() {
						actualLRP := models.NewClaimedActualLRP(models.NewActualLRPKey("some-guid", 0, "some-domain"),
							models.NewActualLRPInstanceKey("instance-guid-1", cellId),
							1,
						)
						actualLRPGroup := models.NewRunningActualLRPGroup(actualLRP)
						expectedEvent = models.NewActualLRPCreatedEvent(actualLRPGroup)
					})

					JustBeforeEach(func() {
						actualHub.Emit(expectedEvent)
					})

					Context("subscriber with the right filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: cellId,
							}
						})

						Context("and the LRP is in an evacuating state", func() {
							BeforeEach(func() {
								actualLRP := models.NewClaimedActualLRP(models.NewActualLRPKey("some-guid", 0, "some-domain"),
									models.NewActualLRPInstanceKey("instance-guid-1", cellId),
									1,
								)
								actualLRPGroup := models.NewEvacuatingActualLRPGroup(actualLRP)
								expectedEvent = models.NewActualLRPCreatedEvent(actualLRPGroup)
							})

							It("receives events from the filtered cell", func() {
								Eventually(eventsCh).Should(Receive(Equal(expectedEvent)))
							})
						})

						It("receives events from the filtered cell", func() {
							Eventually(eventsCh).Should(Receive(Equal(expectedEvent)))
						})
					})

					Context("subscriber with the wrong filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: "another-cell-id",
							}
						})

						It("does not receives events from the filtered cell", func() {
							Consistently(eventsCh).ShouldNot(Receive(Equal(expectedEvent)))
						})
					})
				})

				Context("ActualLRPRemovedEvent", func() {
					var (
						expectedEvent *models.ActualLRPRemovedEvent
					)

					BeforeEach(func() {
						actualLRP := models.NewClaimedActualLRP(models.NewActualLRPKey("some-guid", 0, "some-domain"),
							models.NewActualLRPInstanceKey("instance-guid-1", cellId),
							1,
						)
						actualLRPGroup := models.NewRunningActualLRPGroup(actualLRP)
						expectedEvent = models.NewActualLRPRemovedEvent(actualLRPGroup)
					})

					JustBeforeEach(func() {
						actualHub.Emit(expectedEvent)
					})

					Context("subscriber with the right filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: cellId,
							}
						})

						Context("and the LRP is in an evacuating state", func() {
							BeforeEach(func() {
								actualLRP := models.NewClaimedActualLRP(models.NewActualLRPKey("some-guid", 0, "some-domain"),
									models.NewActualLRPInstanceKey("instance-guid-1", cellId),
									1,
								)
								actualLRPGroup := models.NewEvacuatingActualLRPGroup(actualLRP)
								expectedEvent = models.NewActualLRPRemovedEvent(actualLRPGroup)
							})

							It("receives events from the filtered cell", func() {
								Eventually(eventsCh).Should(Receive(Equal(expectedEvent)))
							})
						})

						It("receives events from the filtered cell", func() {
							Eventually(eventsCh).Should(Receive(Equal(expectedEvent)))
						})
					})

					Context("subscriber with the wrong filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: "another-cell-id",
							}
						})

						It("does not receives events from the filtered cell", func() {
							Consistently(eventsCh).ShouldNot(Receive(Equal(expectedEvent)))
						})
					})
				})

				Context("ActualLRPCrashedEvent", func() {
					var (
						expectedEvent *models.ActualLRPCrashedEvent
					)

					JustBeforeEach(func() {
						actualLRPBefore := models.NewClaimedActualLRP(
							models.NewActualLRPKey("some-guid", 0, "some-domain"),
							models.NewActualLRPInstanceKey("instance-guid-1", cellId),
							1,
						)
						actualLRPAfter := models.NewUnclaimedActualLRP(models.NewActualLRPKey("guid", 0, "some-domain"), 1)

						expectedEvent = models.NewActualLRPCrashedEvent(actualLRPBefore, actualLRPAfter)

						actualHub.Emit(expectedEvent)
					})

					Context("subscriber with the right filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: cellId,
							}
						})

						It("receives events from the filtered cell", func() {
							Eventually(eventsCh).Should(Receive(Equal(expectedEvent)))
						})
					})

					Context("subscriber with the wrong filter", func() {
						BeforeEach(func() {
							requestBody = &models.EventsByCellId{
								CellId: "another-cell-id",
							}
						})

						It("does not receives events from the filtered cell", func() {
							Consistently(eventsCh).ShouldNot(Receive(Equal(expectedEvent)))
						})
					})
				})
			})
		})
	})

	Describe("Tasks Subscribe_r0", func() {
		var (
			taskHub events.Hub
		)

		BeforeEach(func() {
			taskHub = events.NewHub()
			handler = handlers.NewTaskEventHandler(taskHub)
		})

		AfterEach(func() {
			taskHub.Close()
		})

		Describe("Subscribe to Task Events", func() {

			ItStreamsEventsFromHub(&taskHub)
			ItRecoversFromLostConnections(&taskHub)
		})

	})

})

func streamEvents(eventSource events.EventSource) chan models.Event {
	eventChannel := make(chan models.Event)

	go func() {
		for {
			event, err := eventSource.Next()
			if err != nil {
				close(eventChannel)
				return
			}
			eventChannel <- event
		}
	}()

	return eventChannel
}
