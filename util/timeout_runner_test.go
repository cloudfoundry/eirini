package util_test

import (
	"time"

	"code.cloudfoundry.org/eirini/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TimeoutRunner", func() {

	It("should execute the function passed to it", func() {
		wasExecuted := false
		runsForever := func(_ chan<- interface{}, _ <-chan interface{}) {
			wasExecuted = true
			time.Sleep(1 * time.Minute)
		}

		_ = util.RunWithTimeout(runsForever, 1*time.Millisecond)
		Expect(wasExecuted).To(BeTrue())
	})

	When("the function doesn't write to ready chan before timeout", func() {
		It("should return an error with a helpful message", func() {
			runsForever := func(_ chan<- interface{}, _ <-chan interface{}) {
				time.Sleep(1 * time.Minute)
			}

			Expect(util.RunWithTimeout(runsForever, 1*time.Millisecond)).
				To(MatchError("timed out after 1ms"))
		})

		It("should send a stop signal to the function", func() {
			recievedStop := false
			runsForever := func(_ chan<- interface{}, stop <-chan interface{}) {
				<-stop
				recievedStop = true
			}

			_ = util.RunWithTimeout(runsForever, 1*time.Millisecond)
			Eventually(func() bool { return recievedStop }, 1*time.Second).Should(BeTrue())
		})
	})

	When("the function writes to ready chan before timeout", func() {
		It("should return no error", func() {
			runsWithinTime := func(ready chan<- interface{}, stop <-chan interface{}) {
				ready <- nil
			}

			Expect(util.RunWithTimeout(runsWithinTime, 10*time.Millisecond)).To(Succeed())
		})
	})

	When("the function exits without writing to ready chan", func() {
		It("should return an error with a helpful message", func() {
			exitsWithoutTelling := func(ready chan<- interface{}, stop <-chan interface{}) {}

			Expect(util.RunWithTimeout(exitsWithoutTelling, 10*time.Millisecond)).
				To(MatchError("function completed before timeout, but did not write to ready chan"))
		})
	})
})
