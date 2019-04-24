package rootfspatcher_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/rootfspatcher/rootfspatcherfakes"
)

var _ = Describe("PatchAndWait", func() {

	var (
		patcher *rootfspatcherfakes.FakePatcher
		waiter  *rootfspatcherfakes.FakeWaiter
	)

	BeforeEach(func() {
		patcher = &rootfspatcherfakes.FakePatcher{}
		waiter = &rootfspatcherfakes.FakeWaiter{}

	})

	It("should patch and wait", func() {
		err := PatchAndWait(patcher, waiter)
		Expect(err).ToNot(HaveOccurred())
		Expect(patcher.PatchCallCount()).To(Equal(1))
		Expect(waiter.WaitCallCount()).To(Equal(1))
	})

	It("should return an error if the patcher fails", func() {
		patcher.PatchReturns(errors.New("patching failed"))

		err := PatchAndWait(patcher, waiter)

		Expect(err).To(MatchError("failed to patch resources: patching failed"))
		Expect(patcher.PatchCallCount()).To(Equal(1))
		Expect(waiter.WaitCallCount()).To(Equal(0))

	})

	It("should return an error if the waiter fails", func() {
		waiter.WaitReturns(errors.New("waiting failed"))

		err := PatchAndWait(patcher, waiter)

		Expect(err).To(MatchError("failed to wait for update: waiting failed"))
		Expect(patcher.PatchCallCount()).To(Equal(1))
		Expect(waiter.WaitCallCount()).To(Equal(1))
	})
})
