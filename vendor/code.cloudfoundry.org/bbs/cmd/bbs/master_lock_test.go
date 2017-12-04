package main_test

import (
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/clock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/locket"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MasterLock", func() {
	Context("when the bbs cannot obtain the bbs lock", func() {
		var competingBBSLockProcess ifrit.Process

		BeforeEach(func() {
			competingBBSLock := locket.NewLock(logger, consulClient, locket.LockSchemaPath("bbs_lock"), []byte{}, clock.NewClock(), locket.RetryInterval, locket.DefaultSessionTTL, locket.WithMetronClient(&mfakes.FakeIngressClient{}))
			competingBBSLockProcess = ifrit.Invoke(competingBBSLock)

			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsRunner.StartCheck = "bbs.consul-lock.acquiring-lock"

			bbsProcess = ginkgomon.Invoke(bbsRunner)
		})

		AfterEach(func() {
			ginkgomon.Kill(competingBBSLockProcess)
		})

		It("is not reachable", func() {
			_, err := client.Domains(logger)
			Expect(err).To(HaveOccurred())
		})

		It("becomes available when the lock can be acquired", func() {
			ginkgomon.Kill(competingBBSLockProcess)

			Eventually(func() error {
				_, err := client.Domains(logger)
				return err
			}).ShouldNot(HaveOccurred())
		})
	})

	Context("when the bbs loses the master lock", func() {
		BeforeEach(func() {
			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsProcess = ginkgomon.Invoke(bbsRunner)
			consulRunner.Reset()
		})

		It("exits with an error", func() {
			Eventually(bbsRunner.ExitCode, 3).Should(Equal(1))
		})
	})
})
