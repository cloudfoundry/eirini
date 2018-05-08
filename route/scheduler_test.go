package route_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/julz/cube/route"
)

var _ = Describe("Scheduler", func() {

	Describe("TickerTaskScheduler", func() {
		Context("When task is Scheduled", func() {

			var (
				callCount int
				ticker    *time.Ticker
				duration  time.Duration
				task      func() error
				scheduler TaskScheduler
			)

			BeforeEach(func() {
				duration = time.Duration(20) * time.Millisecond
				ticker = time.NewTicker(duration)
				scheduler = &TickerTaskScheduler{Ticker: ticker}
				callCount = 0

				task = func() error {
					callCount++
					return nil
				}
			})

			JustBeforeEach(func() {
				go scheduler.Schedule(task)
			})

			It("should call the provided function on each tick", func() {
				time.Sleep(50 * time.Millisecond)
				ticker.Stop()

				Expect(callCount).To(Equal(2))
			})

		})

	})
})
