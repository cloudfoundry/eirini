package main_test

import (
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("BBS main test", func() {
	JustBeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
	})

	Context("when sql is not configured", func() {
		BeforeEach(func() {
			bbsConfig.DatabaseDriver = ""
			bbsConfig.DatabaseConnectionString = ""
		})

		It("the bbs returns a validation error", func() {
			bbsProcess = ifrit.Invoke(bbsRunner)
			Eventually(bbsProcess.Wait()).Should(Receive(HaveOccurred()))
		})
	})

	Context("when the metron agent isn't up", func() {
		BeforeEach(func() {
			testIngressServer.Stop()
		})

		It("exit with non-zero status code", func() {
			bbsProcess = ifrit.Background(bbsRunner)
			Eventually(bbsProcess.Wait()).Should(Receive(HaveOccurred()))
		})
	})
})
