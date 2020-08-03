package util_test

import (
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/eirini/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Poll", func() {
	It("execute the passed function", func() {
		stop := make(chan interface{}, 1)
		defer close(stop)

		called := false
		f := func() bool {
			called = true

			return true
		}

		Expect(util.PollUntilTrue(f, 1*time.Millisecond, stop)).To(BeTrue())
		Expect(called).To(BeTrue())
	})

	It("execute the passed function until it returns true", func() {
		stop := make(chan interface{}, 1)
		defer close(stop)

		calledTimes := 0
		f := func() bool {
			calledTimes++

			return calledTimes == 2
		}

		Expect(util.PollUntilTrue(f, 1*time.Millisecond, stop)).To(BeTrue())
		Expect(calledTimes).To(Equal(2))
	})

	It("stops executing when asked to stop", func() {
		f := func() bool {
			return false
		}

		pollResult := true

		stop := make(chan interface{}, 1)
		stopped := make(chan interface{}, 1)
		defer close(stop)
		go func() {
			pollResult = util.PollUntilTrue(f, 1*time.Millisecond, stop)
			stopped <- nil
		}()

		stop <- nil
		Eventually(stopped).Should(Receive())
		Expect(pollResult).To(BeFalse())
	})

	It("sleeps for given duration between polls", func() {
		stop := make(chan interface{}, 1)
		defer close(stop)

		var funcCalledTimes int32
		atomic.StoreInt32(&funcCalledTimes, 0)
		f := func() bool {
			atomic.AddInt32(&funcCalledTimes, 1)

			return false
		}

		go func() {
			util.PollUntilTrue(f, 50*time.Millisecond, stop)
		}()

		time.Sleep(130 * time.Millisecond)
		Expect(atomic.LoadInt32(&funcCalledTimes)).To(Equal(int32(2)))
	})
})
