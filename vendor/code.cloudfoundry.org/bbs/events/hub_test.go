package events_test

import (
	"strconv"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hub", func() {
	var (
		hub events.Hub
	)

	BeforeEach(func() {
		hub = events.NewHub()
	})

	Describe("RegisterCallback", func() {
		Context("when registering the callback", func() {
			var eventSource events.EventSource
			var counts chan int

			BeforeEach(func() {
				var err error
				eventSource, err = hub.Subscribe()
				Expect(err).NotTo(HaveOccurred())

				counts = make(chan int, 1)
				cbCounts := counts
				hub.RegisterCallback(func(count int) {
					cbCounts <- count
				})
			})

			It("calls the callback immediately with the current subscriber count", func() {
				Eventually(counts).Should(Receive(Equal(1)))
			})

			Context("when adding another subscriber", func() {
				BeforeEach(func() {
					Eventually(counts).Should(Receive())

					_, err := hub.Subscribe()
					Expect(err).NotTo(HaveOccurred())
				})

				It("calls the callback with the new subscriber count", func() {
					Eventually(counts).Should(Receive(Equal(2)))
				})
			})

			Context("when all subscribers are dropped", func() {
				BeforeEach(func() {
					Eventually(counts).Should(Receive())

					err := eventSource.Close()
					Expect(err).NotTo(HaveOccurred())
				})

				It("calls the callback with a zero count", func() {
					Eventually(counts).Should(Receive(BeZero()))
				})
			})

			Context("when the hub is closed", func() {
				BeforeEach(func() {
					Eventually(counts).Should(Receive())

					err := hub.Close()
					Expect(err).NotTo(HaveOccurred())
				})

				It("calls the callback with a zero count", func() {
					Eventually(counts).Should(Receive(BeZero()))
				})
			})
		})
	})

	It("fans-out events emitted to it to all subscribers", func() {
		source1, err := hub.Subscribe()
		Expect(err).NotTo(HaveOccurred())
		source2, err := hub.Subscribe()
		Expect(err).NotTo(HaveOccurred())

		hub.Emit(eventfakes.FakeEvent{Token: "1"})
		Expect(source1.Next()).To(Equal(eventfakes.FakeEvent{Token: "1"}))
		Expect(source2.Next()).To(Equal(eventfakes.FakeEvent{Token: "1"}))

		hub.Emit(eventfakes.FakeEvent{Token: "2"})
		Expect(source1.Next()).To(Equal(eventfakes.FakeEvent{Token: "2"}))
		Expect(source2.Next()).To(Equal(eventfakes.FakeEvent{Token: "2"}))
	})

	It("closes slow consumers after MAX_PENDING_SUBSCRIBER_EVENTS missed events", func() {
		slowConsumer, err := hub.Subscribe()
		Expect(err).NotTo(HaveOccurred())

		By("filling the 'buffer'")
		for eventToken := 0; eventToken < events.MAX_PENDING_SUBSCRIBER_EVENTS; eventToken++ {
			hub.Emit(eventfakes.FakeEvent{Token: strconv.Itoa(eventToken)})
		}

		By("reading 2 events off")
		ev, err := slowConsumer.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(ev).To(Equal(eventfakes.FakeEvent{Token: "0"}))

		ev, err = slowConsumer.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(ev).To(Equal(eventfakes.FakeEvent{Token: "1"}))

		By("putting 3 more events on, 'overflowing the buffer' and making the consumer 'slow'")
		for eventToken := events.MAX_PENDING_SUBSCRIBER_EVENTS; eventToken < events.MAX_PENDING_SUBSCRIBER_EVENTS+3; eventToken++ {
			hub.Emit(eventfakes.FakeEvent{Token: strconv.Itoa(eventToken)})
		}

		By("reading off all the 'buffered' events")
		for eventToken := 2; eventToken < events.MAX_PENDING_SUBSCRIBER_EVENTS+2; eventToken++ {
			ev, err = slowConsumer.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(ev).To(Equal(eventfakes.FakeEvent{Token: strconv.Itoa(eventToken)}))
		}

		By("trying to read more out of the source")
		_, err = slowConsumer.Next()
		Expect(err).To(Equal(events.ErrReadFromClosedSource))
	})

	Describe("closing an event source", func() {
		It("prevents current events from propagating to the source", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			hub.Emit(eventfakes.FakeEvent{Token: "1"})
			Expect(source.Next()).To(Equal(eventfakes.FakeEvent{Token: "1"}))

			err = source.Close()
			Expect(err).NotTo(HaveOccurred())

			_, err = source.Next()
			Expect(err).To(Equal(events.ErrReadFromClosedSource))
		})

		It("prevents future events from propagating to the source", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			err = source.Close()
			Expect(err).NotTo(HaveOccurred())

			hub.Emit(eventfakes.FakeEvent{Token: "1"})

			_, err = source.Next()
			Expect(err).To(Equal(events.ErrReadFromClosedSource))
		})

		It("immediately removes the closed event source from its subscribers", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			counts := make(chan int, 1)

			hub.RegisterCallback(func(count int) {
				counts <- count
			})

			Eventually(counts).Should(Receive(Equal(1)))

			err = source.Close()
			Expect(err).NotTo(HaveOccurred())

			Eventually(counts).Should(Receive(BeZero()))
		})

		Context("when the source is already closed", func() {
			It("errors", func() {
				source, err := hub.Subscribe()
				Expect(err).NotTo(HaveOccurred())

				err = source.Close()
				Expect(err).NotTo(HaveOccurred())

				err = source.Close()
				Expect(err).To(Equal(events.ErrSourceAlreadyClosed))
			})
		})
	})

	Describe("closing the hub", func() {
		It("all subscribers receive errors", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			err = hub.Close()
			Expect(err).NotTo(HaveOccurred())

			_, err = source.Next()
			Expect(err).To(Equal(events.ErrReadFromClosedSource))
		})

		It("does not accept new subscribers", func() {
			err := hub.Close()
			Expect(err).NotTo(HaveOccurred())

			_, err = hub.Subscribe()
			Expect(err).To(Equal(events.ErrSubscribedToClosedHub))
		})

		Context("when the hub is already closed", func() {
			BeforeEach(func() {
				err := hub.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors", func() {
				err := hub.Close()
				Expect(err).To(Equal(events.ErrHubAlreadyClosed))
			})
		})
	})
})
