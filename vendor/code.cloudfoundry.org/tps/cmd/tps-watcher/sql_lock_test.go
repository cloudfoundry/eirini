package main_test

import (
	"database/sql"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"

	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("SqlLock", func() {
	var (
		locketRunner  ifrit.Runner
		locketProcess ifrit.Process
		locketAddress string
	)

	BeforeEach(func() {
		locketPort, err := localip.LocalPort()
		Expect(err).NotTo(HaveOccurred())

		dbName := fmt.Sprintf("locket_%d", GinkgoParallelNode())
		connectionString := "postgres://locket:locket_pw@localhost"
		db, err := sql.Open("postgres", connectionString+"?sslmode=disable")
		Expect(err).NotTo(HaveOccurred())
		Expect(db.Ping()).NotTo(HaveOccurred())

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		Expect(err).NotTo(HaveOccurred())

		locketAddress = fmt.Sprintf("localhost:%d", locketPort)
		locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
			cfg.ConsulCluster = consulRunner.ConsulCluster()
			cfg.DatabaseConnectionString = connectionString + "/" + dbName
			cfg.DatabaseDriver = "postgres"
			cfg.ListenAddress = locketAddress
		})
		locketProcess = ginkgomon.Invoke(locketRunner)

		watcherConfig.ClientLocketConfig = locketrunner.ClientLocketConfig()
		watcherConfig.ClientLocketConfig.LocketAddress = locketAddress

		fakeBBS.AllowUnhandledRequests = true
	})

	AfterEach(func() {
		ginkgomon.Interrupt(watcher, 5*time.Second)
		ginkgomon.Interrupt(locketProcess, 5*time.Second)
	})

	Context("with invalid configuration", func() {
		Context("and the locket address is not configured", func() {
			BeforeEach(func() {
				watcherConfig.LocketAddress = ""
				watcherConfig.SkipConsulLock = true
				disableStartCheck = true
			})

			It("exits with an error", func() {
				Eventually(runner).Should(gexec.Exit(2))
			})
		})
	})

	Context("with valid configuration", func() {
		It("acquires the lock in locket and becomes active", func() {
			Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("tps-watcher.started"))
		})

		Context("and the locking server becomes unreachable after grabbing the lock", func() {
			JustBeforeEach(func() {
				Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("tps-watcher.started"))

				ginkgomon.Interrupt(locketProcess, 5*time.Second)
			})

			It("exits after the TTL expires", func() {
				Eventually(runner, 16*time.Second).Should(gexec.Exit(1))
			})
		})

		Context("when consul lock isn't required", func() {
			var competingLockProcess ifrit.Process

			BeforeEach(func() {
				watcherConfig.SkipConsulLock = true
				competingLock := locket.NewLock(
					logger,
					consulRunner.NewClient(),
					locket.LockSchemaPath("tps_watcher_lock"),
					[]byte{}, clock.NewClock(),
					locket.RetryInterval,
					locket.DefaultSessionTTL,
				)
				competingLockProcess = ifrit.Invoke(competingLock)
			})

			AfterEach(func() {
				ginkgomon.Kill(competingLockProcess, 5*time.Second)
			})

			It("does not acquire the consul lock", func() {
				Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("tps-watcher.started"))
			})
		})

		Context("when the lock is not available", func() {
			var competingProcess ifrit.Process

			BeforeEach(func() {
				locketClient, err := locket.NewClient(logger, watcherConfig.ClientLocketConfig)
				Expect(err).NotTo(HaveOccurred())

				lockIdentifier := &locketmodels.Resource{
					Key:      "tps_watcher",
					Owner:    "Your worst enemy.",
					Value:    "Something",
					TypeCode: locketmodels.LOCK,
				}

				clock := clock.NewClock()
				competingRunner := lock.NewLockRunner(logger, locketClient, lockIdentifier, 5, clock, locket.RetryInterval)
				competingProcess = ginkgomon.Invoke(competingRunner)

				disableStartCheck = true
			})

			AfterEach(func() {
				ginkgomon.Interrupt(competingProcess, 5*time.Second)
			})

			It("does not become active", func() {
				Consistently(runner.Buffer, 5*time.Second).ShouldNot(gbytes.Say("tps-watcher.started"))
			})

			Context("and the lock becomes available", func() {
				JustBeforeEach(func() {
					Consistently(runner.Buffer, 5*time.Second).ShouldNot(gbytes.Say("tps-watcher.started"))

					ginkgomon.Interrupt(competingProcess, 5*time.Second)
				})

				It("grabs the lock and becomes active", func() {
					Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("tps-watcher.started"))
				})
			})
		})
	})
})
