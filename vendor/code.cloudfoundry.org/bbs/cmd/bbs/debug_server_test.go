package main_test

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Debug Address", func() {
	var debugAddress string

	BeforeEach(func() {
		port := 6800 + GinkgoParallelNode()*2
		debugAddress = fmt.Sprintf("127.0.0.1:%d", port)
		bbsConfig.DebugAddress = debugAddress
	})

	JustBeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
		bbsProcess = ginkgomon.Invoke(bbsRunner)
	})

	It("listens on the debug address specified", func() {
		_, err := net.Dial("tcp", debugAddress)
		Expect(err).NotTo(HaveOccurred())
	})
})
