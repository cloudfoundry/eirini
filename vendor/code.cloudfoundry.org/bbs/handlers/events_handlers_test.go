package handlers_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vito/go-sse/sse"
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

	Describe("LRPGroup events Subscribe_r0", func() {
		var (
			desiredHub events.Hub
			actualHub  events.Hub
		)

		BeforeEach(func() {
			desiredHub = events.NewHub()
			actualHub = events.NewHub()
			handler = handlers.NewLRPGroupEventsHandler(desiredHub, actualHub)
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
				migratedLRP.ImageLayers = nil

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

	Describe("LRPGroup events Subscribe_r1", func() {
		var (
			desiredHub events.Hub
			actualHub  events.Hub
		)

		BeforeEach(func() {
			desiredHub = events.NewHub()
			actualHub = events.NewHub()
			handler = handlers.NewLRPGroupEventsHandler(desiredHub, actualHub)
		})

		AfterEach(func() {
			desiredHub.Close()
			actualHub.Close()
		})

		Describe("Subscribe to Desired Events", func() {
			It("migrates desired lrps down to v3", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Subscribe_r1(logger, w, r)
				}))

				response, err := http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())
				reader := sse.NewReadCloser(response.Body)

				desiredLRP := model_helpers.NewValidDesiredLRP("guid")
				event := models.NewDesiredLRPCreatedEvent(desiredLRP)

				migratedLRP := desiredLRP.VersionDownTo(format.V3)
				Expect(migratedLRP).To(Equal(desiredLRP))
				Expect(migratedLRP.ImageLayers).NotTo(BeEmpty())

				migratedEvent := models.NewDesiredLRPCreatedEvent(migratedLRP)

				desiredHub.Emit(event)

				events := events.NewEventSource(reader)
				actualEvent, err := events.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(actualEvent).To(Equal(migratedEvent))

				server.Close()
			})
		})
	})

	Describe("when there are r0 and r1 subscribers", func() {
		var (
			desiredHub events.Hub
			actualHub  events.Hub
		)

		BeforeEach(func() {
			desiredHub = events.NewHub()
			actualHub = events.NewHub()
			handler = handlers.NewLRPGroupEventsHandler(desiredHub, actualHub)
		})

		// The race occurs when r0 event stream is doing the down conversion
		// and r1 event stream is serializing the same event.
		It("creates a deep copy of the desired lrp to avoid race", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "r1") {
					handler.Subscribe_r1(logger, w, r)
				} else {
					handler.Subscribe_r0(logger, w, r)
				}
			}))

			responseV0, err := http.Get(server.URL + "/v1/events")
			Expect(err).NotTo(HaveOccurred())
			readerV0 := sse.NewReadCloser(responseV0.Body)

			responseV1, err := http.Get(server.URL + "/v1/events.r1")
			Expect(err).NotTo(HaveOccurred())
			readerV1 := sse.NewReadCloser(responseV1.Body)

			desiredLRPV3 := model_helpers.NewValidDesiredLRP("guid")
			eventV1 := models.NewDesiredLRPCreatedEvent(desiredLRPV3)

			desiredLRPV0 := model_helpers.NewValidDesiredLRP("guid").VersionDownTo(format.V0)
			eventV0 := models.NewDesiredLRPCreatedEvent(desiredLRPV0)

			desiredHub.Emit(models.NewDesiredLRPCreatedEvent(model_helpers.NewValidDesiredLRP("guid")))

			eventsV0 := events.NewEventSource(readerV0)
			eventsV1 := events.NewEventSource(readerV1)

			actualEventV0, err := eventsV0.Next()
			Expect(err).NotTo(HaveOccurred())

			actualEventV1, err := eventsV1.Next()
			Expect(err).NotTo(HaveOccurred())

			Expect(actualEventV0).To(test_helpers.DeepEqual(eventV0))
			Expect(actualEventV1).To(test_helpers.DeepEqual(eventV1))
		})
	})

	Describe("Instance Events Subscribe_r0", func() {
		var (
			desiredHub     events.Hub
			lrpInstanceHub events.Hub
		)

		BeforeEach(func() {
			desiredHub = events.NewHub()
			lrpInstanceHub = events.NewHub()
			handler = handlers.NewLRPInstanceEventHandler(desiredHub, lrpInstanceHub)
		})

		AfterEach(func() {
			desiredHub.Close()
			lrpInstanceHub.Close()
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
	})

	Describe("Instance Events Subscribe_r1", func() {
		var (
			desiredHub     events.Hub
			lrpInstanceHub events.Hub
		)

		BeforeEach(func() {
			desiredHub = events.NewHub()
			lrpInstanceHub = events.NewHub()
			handler = handlers.NewLRPInstanceEventHandler(desiredHub, lrpInstanceHub)
		})

		AfterEach(func() {
			desiredHub.Close()
			lrpInstanceHub.Close()
		})

		Describe("Subscribe to Desired Events", func() {
			It("migrates desired lrps down to v3", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Subscribe_r1(logger, w, r)
				}))

				response, err := http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())
				reader := sse.NewReadCloser(response.Body)

				desiredLRP := model_helpers.NewValidDesiredLRP("guid")
				event := models.NewDesiredLRPCreatedEvent(desiredLRP)

				migratedLRP := desiredLRP.VersionDownTo(format.V3)
				Expect(migratedLRP).To(Equal(desiredLRP))
				Expect(migratedLRP.ImageLayers).NotTo(BeEmpty())

				migratedEvent := models.NewDesiredLRPCreatedEvent(migratedLRP)

				desiredHub.Emit(event)

				events := events.NewEventSource(reader)
				actualEvent, err := events.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(actualEvent).To(Equal(migratedEvent))

				server.Close()
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

			Context("downgrading task definitions down to v2", func() {
				var (
					server          *httptest.Server
					task            *models.Task
					downgradedTask  *models.Task
					event           models.Event
					downgradedEvent models.Event
					eventSource     events.EventSource
				)

				BeforeEach(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						handler.Subscribe_r0(logger, w, r)
					}))

					response, err := http.Get(server.URL)
					Expect(err).NotTo(HaveOccurred())
					reader := sse.NewReadCloser(response.Body)
					eventSource = events.NewEventSource(reader)

					task = model_helpers.NewValidTask("guid")

					downgradedTask = task.VersionDownTo(format.V2)
				})

				JustBeforeEach(func() {
					taskHub.Emit(event)
				})

				AfterEach(func() {
					server.Close()
				})

				Context("TaskCreatedEvent", func() {
					BeforeEach(func() {
						event = models.NewTaskCreatedEvent(task)
						downgradedEvent = models.NewTaskCreatedEvent(downgradedTask)
					})

					It("downgrades correctly", func() {
						actualEvent, err := eventSource.Next()
						Expect(err).NotTo(HaveOccurred())
						Expect(actualEvent).To(Equal(downgradedEvent))
					})
				})

				Context("TaskRemovedEvent", func() {
					BeforeEach(func() {
						event = models.NewTaskRemovedEvent(task)
						downgradedEvent = models.NewTaskRemovedEvent(downgradedTask)
					})

					It("downgrades correctly", func() {
						actualEvent, err := eventSource.Next()
						Expect(err).NotTo(HaveOccurred())
						Expect(actualEvent).To(Equal(downgradedEvent))
					})
				})

				Context("TaskChangedEvent", func() {
					BeforeEach(func() {
						event = models.NewTaskChangedEvent(task, task)
						downgradedEvent = models.NewTaskChangedEvent(downgradedTask, downgradedTask)
					})

					It("downgrades correctly", func() {
						actualEvent, err := eventSource.Next()
						Expect(err).NotTo(HaveOccurred())
						Expect(actualEvent).To(Equal(downgradedEvent))
					})
				})
			})
		})
	})

	Describe("Tasks Subscribe_r1", func() {
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
			Context("downgrading task definitions down to v3", func() {
				var (
					server          *httptest.Server
					task            *models.Task
					downgradedTask  *models.Task
					event           models.Event
					downgradedEvent models.Event
					eventSource     events.EventSource
				)

				BeforeEach(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						handler.Subscribe_r1(logger, w, r)
					}))

					response, err := http.Get(server.URL)
					Expect(err).NotTo(HaveOccurred())
					reader := sse.NewReadCloser(response.Body)
					eventSource = events.NewEventSource(reader)

					task = model_helpers.NewValidTask("guid")

					downgradedTask = task.VersionDownTo(format.V3)
					Expect(downgradedTask).To(Equal(task))
					Expect(downgradedTask.ImageLayers).NotTo(BeNil())
				})

				JustBeforeEach(func() {
					taskHub.Emit(event)
				})

				AfterEach(func() {
					server.Close()
				})

				Context("TaskCreatedEvent", func() {
					BeforeEach(func() {
						event = models.NewTaskCreatedEvent(task)
						downgradedEvent = models.NewTaskCreatedEvent(downgradedTask)
					})

					It("downgrades correctly", func() {
						actualEvent, err := eventSource.Next()
						Expect(err).NotTo(HaveOccurred())
						Expect(actualEvent).To(Equal(downgradedEvent))
					})
				})

				Context("TaskRemovedEvent", func() {
					BeforeEach(func() {
						event = models.NewTaskRemovedEvent(task)
						downgradedEvent = models.NewTaskRemovedEvent(downgradedTask)
					})

					It("downgrades correctly", func() {
						actualEvent, err := eventSource.Next()
						Expect(err).NotTo(HaveOccurred())
						Expect(actualEvent).To(Equal(downgradedEvent))
					})
				})

				Context("TaskChangedEvent", func() {
					BeforeEach(func() {
						event = models.NewTaskChangedEvent(task, task)
						downgradedEvent = models.NewTaskChangedEvent(downgradedTask, downgradedTask)
					})

					It("downgrades correctly", func() {
						actualEvent, err := eventSource.Next()
						Expect(err).NotTo(HaveOccurred())
						Expect(actualEvent).To(Equal(downgradedEvent))
					})
				})
			})
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
