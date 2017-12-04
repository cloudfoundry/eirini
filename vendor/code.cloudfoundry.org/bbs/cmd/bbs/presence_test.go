package main_test

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/rep/maintain"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("CellPresence", func() {
	var (
		locketRunner  ifrit.Runner
		locketProcess ifrit.Process
		locketAddress string
	)

	BeforeEach(func() {
		locketPort, err := localip.LocalPort()
		Expect(err).NotTo(HaveOccurred())

		locketAddress = fmt.Sprintf("localhost:%d", locketPort)

		locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
			cfg.ConsulCluster = consulRunner.ConsulCluster()
			cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
			cfg.DatabaseDriver = sqlRunner.DriverName()
			cfg.ListenAddress = locketAddress
		})

		locketProcess = ginkgomon.Invoke(locketRunner)

		bbsConfig.ClientLocketConfig = locketrunner.ClientLocketConfig()
		bbsConfig.LocketAddress = locketAddress
	})

	JustBeforeEach(func() {
		bbsRunner = testrunner.WaitForMigration(bbsBinPath, bbsConfig)
		// Give the BBS enough time to start
		bbsRunner.StartCheckTimeout = 4 * locket.RetryInterval
		bbsProcess = ginkgomon.Invoke(bbsRunner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(bbsProcess)
		ginkgomon.Interrupt(locketProcess)
	})

	Context("Cells", func() {
		var (
			cellPresenceLocket, cellPresenceConsul ifrit.Process
			presenceLocket, presenceConsul         *models.CellPresence
		)

		BeforeEach(func() {
			clock := clock.NewClock()
			presenceConsul = &models.CellPresence{
				CellId:     "cell-consul",
				RepAddress: "cell-consul-address",
				RepUrl:     "http://cell-consul-url",
				Zone:       "consul-zone",
				Capacity:   &models.CellCapacity{1, 2, 3},
			}

			cellPresenceClient := maintain.NewCellPresenceClient(consulClient, clock)
			cellPresenceConsul = ifrit.Invoke(cellPresenceClient.NewCellPresenceRunner(
				logger,
				presenceConsul,
				locket.RetryInterval,
				locket.DefaultSessionTTL,
			))

			locketClient, err := locket.NewClient(logger, bbsConfig.ClientLocketConfig)
			Expect(err).NotTo(HaveOccurred())

			presenceLocket = &models.CellPresence{
				CellId:     "cell-locket",
				RepAddress: "cell-locket-address",
				RepUrl:     "https://cell-locket-url",
				Zone:       "locket-zone",
				Capacity:   &models.CellCapacity{4, 5, 6},
			}

			data, err := json.Marshal(presenceLocket)
			Expect(err).NotTo(HaveOccurred())

			lockIdentifier := &locketmodels.Resource{
				Key:      "cell-locket",
				Owner:    "anything",
				Value:    string(data),
				TypeCode: locketmodels.PRESENCE,
			}

			cellPresenceLocket = ginkgomon.Invoke(
				lock.NewPresenceRunner(
					logger,
					locketClient,
					lockIdentifier,
					int64(locket.DefaultSessionTTL/time.Second),
					clock,
					locket.RetryInterval,
				),
			)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(cellPresenceLocket)
			ginkgomon.Interrupt(cellPresenceConsul)
		})

		It("returns cell presences from both locket and consul", func() {
			presences, err := client.Cells(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(presences).To(ConsistOf(presenceLocket, presenceConsul))
		})

		Context("when detect consul cell registrations is false", func() {
			BeforeEach(func() {
				bbsConfig.DetectConsulCellRegistrations = false
			})

			It("only returns cell presences from locket", func() {
				presences, err := client.Cells(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(presences).To(ConsistOf(presenceLocket))
			})
		})
	})
})
