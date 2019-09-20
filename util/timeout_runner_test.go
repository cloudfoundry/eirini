package util_test

import (
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/eirini/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TimeoutRunner", func() {

	It("should execute the function passed to it", func() {
		var callCount int32
		atomic.StoreInt32(&callCount, 0)
		runsForever := func(_ <-chan interface{}) {
			atomic.AddInt32(&callCount, 1)
		}

		_ = util.RunWithTimeout(runsForever, 1*time.Millisecond)
		wasExecutedOnce := func() bool { return atomic.LoadInt32(&callCount) == 1 }
		Eventually(wasExecutedOnce).Should(BeTrue())
	})

	When("the function doesn't complete before timeout", func() {
		It("should return an error with a helpful message", func() {
			runsForever := func(_ <-chan interface{}) {
				time.Sleep(1 * time.Minute)
			}

			Expect(util.RunWithTimeout(runsForever, 1*time.Millisecond)).
				To(MatchError("timed out after 1ms"))
		})

		It("should send a stop signal to the function", func() {
			var recievedStop int32
			atomic.StoreInt32(&recievedStop, 0)
			runsForever := func(stop <-chan interface{}) {
				<-stop
				atomic.AddInt32(&recievedStop, 1)
			}

			_ = util.RunWithTimeout(runsForever, 1*time.Millisecond)
			recievedStopOnce := func() bool { return atomic.LoadInt32(&recievedStop) == 1 }
			Eventually(recievedStopOnce, 1*time.Second).Should(BeTrue())
		})
	})

	When("the function completes before timeout", func() {
		It("should return no error", func() {
			runsWithinTime := func(stop <-chan interface{}) {}

			Expect(util.RunWithTimeout(runsWithinTime, 20*time.Millisecond)).To(Succeed())
		})
	})
})
