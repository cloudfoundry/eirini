package events_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/events/eventsfakes"
	"code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

var _ = Describe("Crashreporter", func() {

	var (
		work          chan CrashReport
		scheduler     *routefakes.FakeTaskScheduler
		crashReporter *CrashReporter
		ccClient      *eventsfakes.FakeCcClient
		crashReports  CrashReport
		err           error
	)

	BeforeEach(func() {
		scheduler = new(routefakes.FakeTaskScheduler)
		work = make(chan CrashReport, 1)
		ccClient = new(eventsfakes.FakeCcClient)
		crashReporter = NewCrashReporter(work, scheduler, ccClient, lagertest.NewTestLogger("tester"))

		crashReports = CrashReport{
			ProcessGuid: "some-guid",
			AppCrashedRequest: cc_messages.AppCrashedRequest{
				Instance:        "0",
				Index:           0,
				Reason:          "who-knows",
				ExitStatus:      1,
				ExitDescription: "fail",
				CrashCount:      3,
				CrashTimestamp:  112233,
			},
		}
	})

	Context("When an app crashes", func() {
		JustBeforeEach(func() {
			crashReporter.Run()

			work <- crashReports

			reportFunc := scheduler.ScheduleArgsForCall(0)
			err = reportFunc()
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should report the app to CC", func() {
			count := ccClient.AppCrashedCallCount()
			Expect(count).To(Equal(1))
		})

		It("should report the right process guid for the first crashed app", func() {
			guid, _, _ := ccClient.AppCrashedArgsForCall(0)
			Expect(guid).To(Equal("some-guid"))
		})

		It("should report the right information for the first crashed app", func() {
			_, report, _ := ccClient.AppCrashedArgsForCall(0)
			Expect(report.Reason).To(Equal("who-knows"))
			Expect(report.CrashTimestamp).To(Equal(int64(112233)))
		})

		Context("event could not be submitted", func() {

			BeforeEach(func() {
				ccClient.AppCrashedReturns(errors.New("boom"))
			})

			It("should return the error", func() {
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})
	})
})
