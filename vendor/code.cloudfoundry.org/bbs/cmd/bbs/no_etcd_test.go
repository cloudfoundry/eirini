package main_test

import (
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/durationjson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("BBS With Only SQL", func() {
	BeforeEach(func() {
		port, err := strconv.Atoi(strings.TrimPrefix(testMetricsListener.LocalAddr().String(), "127.0.0.1:"))
		Expect(err).NotTo(HaveOccurred())

		bbsConfig = config.BBSConfig{
			ListenAddress:     bbsAddress,
			HealthAddress:     bbsHealthAddress,
			AdvertiseURL:      bbsURL.String(),
			AuctioneerAddress: auctioneerServer.URL(),
			ConsulCluster:     consulRunner.ConsulCluster(),
			DropsondePort:     port,
			ReportInterval:    durationjson.Duration(10 * time.Millisecond),
			EncryptionConfig: encryption.EncryptionConfig{
				EncryptionKeys: map[string]string{"label": "key"},
				ActiveKeyLabel: "label",
			},
			ETCDConfig: config.ETCDConfig{},
		}
	})

	JustBeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
	})

	Context("when etcd is partially configured", func() {
		BeforeEach(func() {
			bbsConfig.ETCDConfig.CaFile = "I am a ca cert"
		})

		It("returns a validation error", func() {
			bbsProcess = ifrit.Invoke(bbsRunner)
			Eventually(bbsProcess.Wait()).Should(Receive(HaveOccurred()))
		})
	})

	Context("when etcd is not configured at all", func() {
		Context("and sql is configured", func() {
			BeforeEach(func() {
				bbsConfig.DatabaseDriver = sqlRunner.DriverName()
				bbsConfig.DatabaseConnectionString = sqlRunner.ConnectionString()
			})

			It("the bbs eventually responds to ping", func() {
				bbsProcess = ginkgomon.Invoke(bbsRunner)
				Expect(client.Ping(logger)).To(BeTrue())
			})
		})

		Context("when sql is not configured", func() {
			It("the bbs returns a validation error", func() {
				bbsProcess = ifrit.Invoke(bbsRunner)
				Eventually(bbsProcess.Wait()).Should(Receive(HaveOccurred()))
			})
		})
	})
})
