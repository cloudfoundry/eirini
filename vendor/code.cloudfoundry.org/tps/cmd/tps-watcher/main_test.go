package main_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

const watcherLockName = "tps_watcher_lock"

var _ = Describe("TPS", func() {
	var (
		domain string
	)

	BeforeEach(func() {
		domain = cc_messages.AppLRPDomain
	})

	AfterEach(func() {
		if watcher != nil {
			watcher.Signal(os.Kill)
			Eventually(watcher.Wait()).Should(Receive())
		}
	})

	Describe("Crashed Apps", func() {
		var (
			ready chan struct{}
		)

		BeforeEach(func() {
			ready = make(chan struct{})
			fakeCC.RouteToHandler("POST", "/internal/v4/apps/some-process-guid/crashed", func(res http.ResponseWriter, req *http.Request) {
				var appCrashed cc_messages.AppCrashedRequest

				bytes, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				req.Body.Close()

				err = json.Unmarshal(bytes, &appCrashed)
				Expect(err).NotTo(HaveOccurred())

				Expect(appCrashed.CrashTimestamp).NotTo(BeZero())
				appCrashed.CrashTimestamp = 0

				Expect(appCrashed).To(Equal(cc_messages.AppCrashedRequest{
					Instance:        "some-instance-guid-1",
					Index:           1,
					CellID:          "cell-id",
					Reason:          "CRASHED",
					ExitDescription: "out of memory",
					CrashCount:      1,
				}))

				close(ready)
			})

			lrpKey := models.NewActualLRPKey("some-process-guid", 1, domain)
			instanceKey := models.NewActualLRPInstanceKey("some-instance-guid-1", "cell-id")
			netInfo := models.NewActualLRPNetInfo("1.2.3.4", "5.6.7.8", models.NewPortMapping(65100, 8080))
			beforeActualLRP := *models.NewRunningActualLRP(lrpKey, instanceKey, netInfo, 0)
			afterActualLRP := beforeActualLRP
			afterActualLRP.State = models.ActualLRPStateCrashed
			afterActualLRP.Since = int64(1)
			afterActualLRP.CrashCount = 1
			afterActualLRP.CrashReason = "out of memory"

			fakeBBS.RouteToHandler("GET", "/v1/events.r1",
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher := w.(http.Flusher)
					flusher.Flush()
					closeNotifier := w.(http.CloseNotifier).CloseNotify()
					event := models.NewActualLRPCrashedEvent(&beforeActualLRP, &afterActualLRP)

					sseEvent, err := events.NewEventFromModelEvent(0, event)
					Expect(err).NotTo(HaveOccurred())

					err = sseEvent.Write(w)
					Expect(err).NotTo(HaveOccurred())

					flusher.Flush()

					<-closeNotifier
				},
			)
		})

		It("POSTs to the CC that the application has crashed", func() {
			Eventually(ready, 5*time.Second).Should(BeClosed())
		})
	})

	Context("when the watcher loses the lock", func() {
		BeforeEach(func() {
			fakeBBS.RouteToHandler("GET", "/v1/events.r1",
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)
				},
			)
		})

		JustBeforeEach(func() {
			consulRunner.Reset()
		})

		AfterEach(func() {
			ginkgomon.Interrupt(watcher, 5)
		})

		It("exits with an error", func() {
			Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("lock lost"))
		})
	})

	Context("when the watcher initially does not have the lock", func() {
		var competingWatcherProcess ifrit.Process

		BeforeEach(func() {
			fakeBBS.RouteToHandler("GET", "/v1/events.r1",
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)
				},
			)

			competingWatcher := locket.NewLock(
				logger,
				consulRunner.NewClient(),
				locket.LockSchemaPath(watcherLockName),
				[]byte("something-else"),
				clock.NewClock(),
				locket.RetryInterval,
				locket.DefaultSessionTTL,
			)
			competingWatcherProcess = ifrit.Invoke(competingWatcher)

			disableStartCheck = true
		})

		AfterEach(func() {
			ginkgomon.Interrupt(watcher, 5)
			ginkgomon.Kill(competingWatcherProcess)
		})

		It("does not start", func() {
			Consistently(runner.Buffer, 5*time.Second).ShouldNot(gbytes.Say("tps-watcher.started"))
		})

		Context("when the lock becomes available", func() {
			BeforeEach(func() {
				ginkgomon.Kill(competingWatcherProcess)
			})

			It("is updated", func() {
				Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("tps-watcher.started"))
			})
		})
	})
})
