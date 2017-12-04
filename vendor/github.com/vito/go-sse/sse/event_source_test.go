package sse_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type failedThenSucceedsRoundTripper struct {
	failCount   int
	timesToFail int
}

func (f *failedThenSucceedsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.failCount < f.timesToFail {
		f.failCount++
		return nil, fmt.Errorf("failed %d times (will fail %d times)", f.failCount, f.timesToFail)
	}

	return http.DefaultTransport.RoundTrip(req)
}

type failingRoundTripper struct {
	cb func()
}

func (f *failingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.cb()
	return nil, errors.New("failed to connect")
}

var _ = Describe("EventSource", func() {
	var (
		server *ghttp.Server

		source *EventSource
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		url := server.URL()

		source = NewEventSource(
			http.DefaultClient,
			100*time.Millisecond,
			func() *http.Request {
				request, err := http.NewRequest("GET", url, nil)
				Ω(err).ShouldNot(HaveOccurred())

				return request
			},
		)
	})

	Context("when closing an unused EventSource", func() {
		var err error

		BeforeEach(func() {
			err = source.Close()
		})

		It("does not return an error", func() {
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when the connection breaks after events are read", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				func(w http.ResponseWriter, r *http.Request) {
					flusher := w.(http.Flusher)
					closeNotify := w.(http.CloseNotifier).CloseNotify()

					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher.Flush()

					Event{
						ID:   "1",
						Data: []byte("hello"),
					}.Write(w)

					flusher.Flush()

					Event{
						ID:   "2",
						Data: []byte("hello again"),
					}.Write(w)

					flusher.Flush()

					<-closeNotify
				},
				ghttp.CombineHandlers(
					ghttp.VerifyHeaderKV("Last-Event-ID", "2"),
					func(w http.ResponseWriter, r *http.Request) {
						flusher := w.(http.Flusher)

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()

						Event{
							ID:   "3",
							Data: []byte("welcome back"),
						}.Write(w)

						flusher.Flush()
					},
				),
			)
		})

		It("reconnects from the last event id", func() {
			Ω(source.Next()).Should(Equal(Event{
				ID:   "1",
				Data: []byte("hello"),
			}))

			Ω(source.Next()).Should(Equal(Event{
				ID:   "2",
				Data: []byte("hello again"),
			}))

			server.CloseClientConnections()

			Ω(source.Next()).Should(Equal(Event{
				ID:   "3",
				Data: []byte("welcome back"),
			}))

			_, err := source.Next()
			Ω(err).Should(Equal(io.EOF))
		})
	})

	Context("when reconnecting continuously fails", func() {
		var retryTimes <-chan time.Time

		BeforeEach(func() {
			times := make(chan time.Time, 10)
			retryTimes = times

			server.AppendHandlers(
				func(w http.ResponseWriter, r *http.Request) {
					flusher := w.(http.Flusher)

					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher.Flush()

					Event{
						ID:   "1",
						Data: []byte("hello"),
					}.Write(w)

					flusher.Flush()

					server.CloseClientConnections()
				},
				func(w http.ResponseWriter, r *http.Request) {
					times <- time.Now()
					server.CloseClientConnections()
				},
				func(w http.ResponseWriter, r *http.Request) {
					times <- time.Now()
					server.CloseClientConnections()
				},
				func(w http.ResponseWriter, r *http.Request) {
					times <- time.Now()
					server.CloseClientConnections()
				},
				func(w http.ResponseWriter, r *http.Request) {
					times <- time.Now()

					flusher := w.(http.Flusher)

					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher.Flush()

					Event{
						ID:   "2",
						Data: []byte("welcome back"),
					}.Write(w)

					flusher.Flush()

					Event{
						ID:    "3",
						Data:  []byte("see you in a bit"),
						Retry: 200 * time.Millisecond,
					}.Write(w)

					flusher.Flush()

					server.CloseClientConnections()
				},
				func(w http.ResponseWriter, r *http.Request) {
					times <- time.Now()
					server.CloseClientConnections()
				},
				func(w http.ResponseWriter, r *http.Request) {
					times <- time.Now()

					flusher := w.(http.Flusher)

					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher.Flush()

					Event{
						ID:   "4",
						Data: []byte("hello again"),
					}.Write(w)

					flusher.Flush()
				},
			)
		})

		It("retries on the base interval, followed by the server-specified interval", func() {
			var time1, time2, time3, time4, time5 time.Time

			Ω(source.Next()).Should(Equal(Event{
				ID:   "1",
				Data: []byte("hello"),
			}))

			Ω(source.Next()).Should(Equal(Event{
				ID:   "2",
				Data: []byte("welcome back"),
			}))

			Ω(retryTimes).Should(Receive(&time1))
			Ω(retryTimes).Should(Receive(&time2))
			Ω(retryTimes).Should(Receive(&time3))

			Ω(source.Next()).Should(Equal(Event{
				ID:    "3",
				Data:  []byte("see you in a bit"),
				Retry: 200 * time.Millisecond,
			}))

			Ω(retryTimes).Should(Receive(&time4))

			Ω(source.Next()).Should(Equal(Event{
				ID:   "4",
				Data: []byte("hello again"),
			}))

			Ω(retryTimes).Should(Receive(&time5))

			Ω(time5.Sub(time4)).Should(BeNumerically("~", 200*time.Millisecond, 50*time.Millisecond))
			Ω(time4.Sub(time3)).Should(BeNumerically("~", 100*time.Millisecond, 50*time.Millisecond))
			Ω(time3.Sub(time2)).Should(BeNumerically("~", 100*time.Millisecond, 50*time.Millisecond))
			Ω(time2.Sub(time1)).Should(BeNumerically("~", 100*time.Millisecond, 50*time.Millisecond))
		})
	})

	Context("when the server returns 404", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.RespondWith(404, ""),
			)
		})

		It("returns a BadResponseError", func() {
			_, err := source.Next()
			Ω(err).Should(BeAssignableToTypeOf(BadResponseError{}))
		})
	})

	Describe("handling errors while reading events", func() {
		var eventChan chan Event
		var errChan chan error

		BeforeEach(func() {
			eventChan = make(chan Event, 1)
			errChan = make(chan error, 1)
		})

		Context("when the event reader is closed", func() {
			BeforeEach(func() {
				flushedChan := make(chan struct{})

				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						closeNotify := w.(http.CloseNotifier).CloseNotify()
						flusher := w.(http.Flusher)

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()
						close(flushedChan)
						<-closeNotify
					},
					func(w http.ResponseWriter, r *http.Request) {
						flusher := w.(http.Flusher)

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()

						Event{
							ID:   "2",
							Data: []byte("hello again"),
						}.Write(w)

						flusher.Flush()
					},
				)

				doneCh := make(chan struct{})
				go func() {
					event, err := source.Next()
					if err != nil {
						errChan <- err
					} else {
						eventChan <- event
					}
					close(doneCh)
				}()

				<-flushedChan
				err := source.Close()
				Ω(err).ShouldNot(HaveOccurred())
				<-doneCh
			})

			It("returns ErrSourceClosed", func() {
				Eventually(errChan).Should(Receive(Equal(ErrSourceClosed)))
			})

			Context("when calling Close again", func() {
				It("does not return an error", func() {
					err := source.Close()
					Ω(err).ShouldNot(HaveOccurred())
				})
			})

			Context("when trying to call Next again", func() {
				var err error

				BeforeEach(func() {
					_, err = source.Next()
				})

				It("immediately returns ErrSourceClosed", func() {
					Ω(err).Should(Equal(ErrSourceClosed))
				})
			})
		})

		Context("when there are no more events", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						flusher := w.(http.Flusher)

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()
					},
					func(w http.ResponseWriter, r *http.Request) {
						flusher := w.(http.Flusher)
						closeNotify := w.(http.CloseNotifier).CloseNotify()

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()

						Event{
							ID:   "2",
							Data: []byte("hello again"),
						}.Write(w)

						flusher.Flush()

						<-closeNotify
					},
				)

				doneCh := make(chan struct{})
				go func() {
					event, err := source.Next()
					if err != nil {
						errChan <- err
					} else {
						eventChan <- event
					}
					close(doneCh)
				}()

				<-doneCh
			})

			It("returns io.EOF", func() {
				Eventually(errChan).Should(Receive(Equal(io.EOF)))
			})
		})
	})

	Context("when max retries is specified", func() {
		var (
			retryParams RetryParams
		)
		BeforeEach(func() {
			retryParams = RetryParams{
				MaxRetries:    3,
				RetryInterval: 100 * time.Millisecond,
			}
		})

		Context("when event source is unavailable during initial connection", func() {
			It("retries for specified max retries and returns an error", func() {
				actualRetries := uint16(0)
				config := Config{
					RetryParams: retryParams,
					RequestCreator: func() *http.Request {
						request, err := http.NewRequest("GET", "http://something.non.existent", nil)
						Ω(err).ShouldNot(HaveOccurred())
						actualRetries++
						return request
					},
				}
				src, err := config.Connect()
				Ω(err).To(HaveOccurred())
				Ω(src).To(BeNil())
				Ω(actualRetries).To(Equal(retryParams.MaxRetries + 1))
			})
		})

		Context("when event source becomes unavailable after initial connection", func() {
			Context("and stays unavailable for more than max retries", func() {
				var (
					localServer *ghttp.Server
				)
				BeforeEach(func() {
					localServer = ghttp.NewServer()
					retryParams = RetryParams{
						MaxRetries:    2,
						RetryInterval: 100 * time.Millisecond,
					}

					localServer.AppendHandlers(
						func(w http.ResponseWriter, r *http.Request) {
							flusher := w.(http.Flusher)

							w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
							w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Add("Connection", "keep-alive")

							w.WriteHeader(http.StatusOK)

							flusher.Flush()
							localServer.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							localServer.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							localServer.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							localServer.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							flusher := w.(http.Flusher)

							w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
							w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Add("Connection", "keep-alive")

							w.WriteHeader(http.StatusOK)

							flusher.Flush()

							Event{
								ID:   "2",
								Data: []byte("welcome back"),
							}.Write(w)

							flusher.Flush()

							Event{
								ID:    "3",
								Data:  []byte("see you in a bit"),
								Retry: 200 * time.Millisecond,
							}.Write(w)

							flusher.Flush()
						},
					)
				})

				It("retries for specified max retries and returns an error", func() {
					actualRetries := uint16(0)
					config := Config{
						Client:      http.DefaultClient,
						RetryParams: retryParams,
						RequestCreator: func() *http.Request {
							request, err := http.NewRequest("GET", localServer.URL(), nil)
							Ω(err).ShouldNot(HaveOccurred())
							actualRetries++
							return request
						},
					}
					src, err := config.Connect()

					Ω(err).NotTo(HaveOccurred())
					Ω(src).NotTo(BeNil())
					_, err = src.Next()
					Ω(err).To(HaveOccurred())
					// Because actualRetries will be bumped by one during initial Connect
					Ω(actualRetries).To(Equal(retryParams.MaxRetries + 2))
				})
			})

			Context("and becomes available before max retries are reached", func() {
				BeforeEach(func() {
					retryParams = RetryParams{
						MaxRetries:    3,
						RetryInterval: 100 * time.Millisecond,
					}
					server.AppendHandlers(
						func(w http.ResponseWriter, r *http.Request) {
							flusher := w.(http.Flusher)

							w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
							w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Add("Connection", "keep-alive")

							w.WriteHeader(http.StatusOK)

							flusher.Flush()

							server.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							server.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							server.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							server.CloseClientConnections()
						},
						func(w http.ResponseWriter, r *http.Request) {
							flusher := w.(http.Flusher)

							w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
							w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Add("Connection", "keep-alive")

							w.WriteHeader(http.StatusOK)

							flusher.Flush()

							Event{
								ID:   "2",
								Data: []byte("welcome back"),
							}.Write(w)

							flusher.Flush()

							Event{
								ID:    "3",
								Data:  []byte("see you in a bit"),
								Retry: 200 * time.Millisecond,
							}.Write(w)

							flusher.Flush()
						},
					)
				})

				It("returns event", func() {
					actualRetries := uint16(0)
					config := Config{
						Client:      http.DefaultClient,
						RetryParams: retryParams,
						RequestCreator: func() *http.Request {
							request, err := http.NewRequest("GET", server.URL(), nil)
							Ω(err).ShouldNot(HaveOccurred())
							actualRetries++
							return request
						},
					}
					src, err := config.Connect()

					Ω(err).NotTo(HaveOccurred())
					Ω(src).NotTo(BeNil())
					Ω(src.Next()).Should(Equal(Event{
						ID:   "2",
						Data: []byte("welcome back"),
					}))
					Ω(actualRetries).To(Equal(retryParams.MaxRetries + 2))
				})
			})
		})
	})

	for _, retryableStatus := range []int{
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		status := retryableStatus

		Context(fmt.Sprintf("when the server returns %d", status), func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.RespondWith(status, ""),
					func(w http.ResponseWriter, r *http.Request) {
						flusher := w.(http.Flusher)

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()

						Event{
							ID:   "1",
							Data: []byte("you made it!"),
						}.Write(w)

						flusher.Flush()
					},
				)
			})

			It("transparently reconnects", func() {
				Ω(source.Next()).Should(Equal(Event{
					ID:   "1",
					Data: []byte("you made it!"),
				}))
			})
		})
	}

})
