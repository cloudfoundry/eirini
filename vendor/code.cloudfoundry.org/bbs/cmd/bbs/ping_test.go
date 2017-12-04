package main_test

import (
	"net/http"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/clock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/locket"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ping API", func() {
	var fakeMetronClient = &mfakes.FakeIngressClient{}

	Describe("Protobuf Ping", func() {
		It("returns true when the bbs is running", func() {
			By("having the bbs down", func() {
				Expect(client.Ping(logger)).To(BeFalse())
			})

			By("starting the bbs without a lock", func() {
				competingBBSLock := locket.NewLock(logger, consulClient, locket.LockSchemaPath("bbs_lock"), []byte{}, clock.NewClock(), locket.RetryInterval, locket.DefaultSessionTTL, locket.WithMetronClient(fakeMetronClient))
				competingBBSLockProcess := ifrit.Invoke(competingBBSLock)
				defer ginkgomon.Kill(competingBBSLockProcess)

				bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
				bbsRunner.StartCheck = "bbs.consul-lock.acquiring-lock"
				bbsProcess = ginkgomon.Invoke(bbsRunner)

				Expect(client.Ping(logger)).To(BeFalse())
			})

			By("finally acquiring the lock", func() {
				Eventually(func() bool {
					return client.Ping(logger)
				}).Should(BeTrue())
			})
		})
	})

	Describe("HTTP Ping", func() {
		It("returns true when the bbs is running", func() {
			var ping = func() bool {
				resp, err := http.Get("http://" + bbsHealthAddress + "/ping")
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return true
				} else {
					return false
				}
			}

			By("having the bbs down", func() {
				Eventually(ping).Should(BeFalse())
			})

			By("starting the bbs without a lock", func() {
				competingBBSLock := locket.NewLock(logger, consulClient, locket.LockSchemaPath("bbs_lock"), []byte{}, clock.NewClock(), locket.RetryInterval, locket.DefaultSessionTTL, locket.WithMetronClient(fakeMetronClient))
				competingBBSLockProcess := ifrit.Invoke(competingBBSLock)
				defer ginkgomon.Kill(competingBBSLockProcess)

				bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
				bbsRunner.StartCheck = "bbs.consul-lock.acquiring-lock"
				bbsProcess = ginkgomon.Invoke(bbsRunner)

				Eventually(ping).Should(BeTrue())
			})
		})
	})
})
