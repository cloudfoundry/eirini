package util_test

import (
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/util"
)

var _ = Describe("Scheduler", func() {

	Describe("TickerTaskScheduler", func() {
		Context("When task is Scheduled", func() {

			var (
				ticker   *time.Ticker
				duration time.Duration
				count    int32
			)

			BeforeEach(func() {
				duration = time.Duration(20) * time.Millisecond
				ticker = time.NewTicker(duration)
			})

			It("should call the provided function on each tick", func() {
				scheduler := &TickerTaskScheduler{Ticker: ticker}
				task := func() error {
					atomic.AddInt32(&count, 1)
					return nil
				}
				go scheduler.Schedule(task)
				time.Sleep(50 * time.Millisecond)
				ticker.Stop()

				Expect(atomic.LoadInt32(&count)).To(Equal(int32(2)))
			})

		})

	})
})
