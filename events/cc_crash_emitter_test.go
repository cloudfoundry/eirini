package events_test

import (
	"errors"

	. "code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/events/eventsfakes"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crashreporter", func() {
	var (
		crashEmitter *CcCrashEmitter
		ccClient     *eventsfakes.FakeCcClient
		crashEvent   CrashEvent
		err          error
	)

	BeforeEach(func() {
		ccClient = new(eventsfakes.FakeCcClient)
		crashEmitter = NewCcCrashEmitter(tests.NewTestLogger("tester"), ccClient)

		crashEvent = CrashEvent{
			ProcessGUID: "some-guid",
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
			err = crashEmitter.Emit(crashEvent)
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
