package instrumented_round_tripper_test

import (
	"errors"
	"net/http"
	"reflect"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/instrumented_round_tripper"
	"github.com/cloudfoundry/sonde-go/events"
	uuid "github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstrumentedRoundTripper", func() {
	var (
		fakeRoundTripper *FakeRoundTripper
		rt               http.RoundTripper
		req              *http.Request
		fakeEmitter      *fake.FakeEventEmitter
		requestUUID      *uuid.UUID

		origin = "testRoundtripper/42"
	)

	BeforeEach(func() {
		var err error
		fakeEmitter = fake.NewFakeEventEmitter(origin)
		requestUUID, err = uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		fakeRoundTripper = &FakeRoundTripper{}
		rt = instrumented_round_tripper.InstrumentedRoundTripper(fakeRoundTripper, fakeEmitter)

		req, err = http.NewRequest("GET", "http://foo.example.com/", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1"
		req.Header.Set("User-Agent", "our-testing-client")
		req.Header.Set("X-Vcap-Request-Id", requestUUID.String())
	})

	Context("when the round tripper is a cancelable round tripper", func() {
		var fcrt *fakeCancelableRoundTripper
		BeforeEach(func() {
			fcrt = &fakeCancelableRoundTripper{}
			rt = instrumented_round_tripper.InstrumentedRoundTripper(fcrt, fakeEmitter)
		})

		It("returns an instrumentedCancelableRoundTripper", func() {
			Expect(reflect.TypeOf(rt).Elem().Name()).To(Equal("instrumentedCancelableRoundTripper"))

			_, ok := rt.(canceler)
			Expect(ok).To(BeTrue())

			_, ok = rt.(http.RoundTripper)
			Expect(ok).To(BeTrue())
		})

		It("delegates CancelRequest", func() {
			Expect(fcrt.canceled).To(BeFalse())

			c := rt.(canceler)

			c.CancelRequest(nil)
			Expect(fcrt.canceled).To(BeTrue())
		})
	})

	Context("when the round tripper is not a cancelable round tripper", func() {
		BeforeEach(func() {
			fakeRoundTripper = &FakeRoundTripper{}
			rt = instrumented_round_tripper.InstrumentedRoundTripper(fakeRoundTripper, fakeEmitter)
		})

		It("returns an instrumentedRoundTripper", func() {
			Expect(reflect.TypeOf(rt).Elem().Name()).To(Equal("instrumentedRoundTripper"))

			_, ok := rt.(http.RoundTripper)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("request ID", func() {
		It("forwards the request id", func() {
			_, err := rt.RoundTrip(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(req.Header.Get("X-Vcap-Request-Id")).To(Equal(requestUUID.String()))
		})

		It("emits an HttpStartStop event with the request ID", func() {
			_, err := rt.RoundTrip(req)
			Expect(err).ToNot(HaveOccurred())
			startStopEvent := fakeEmitter.GetMessages()[0].Event.(*events.HttpStartStop)
			Expect(startStopEvent.GetRequestId()).To(Equal(factories.NewUUID(requestUUID)))
		})

		Context("if there is no request ID", func() {
			BeforeEach(func() {
				req.Header.Set("X-Vcap-Request-Id", "")
			})

			It("populates the request Id with a new guid", func() {
				_, err := rt.RoundTrip(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(req.Header.Get("X-Vcap-Request-Id")).NotTo(Equal(""))
			})
		})
	})

	Context("event emission", func() {
		Context("if round tripper does not return an error", func() {
			It("should emit a startstop event with the round tripper's response", func() {
				rt.RoundTrip(req)

				Expect(fakeEmitter.GetMessages()[0].Event).To(BeAssignableToTypeOf(new(events.HttpStartStop)))

				startStopEvent := fakeEmitter.GetMessages()[0].Event.(*events.HttpStartStop)
				Expect(startStopEvent.GetStatusCode()).To(BeNumerically("==", 123))
				Expect(startStopEvent.GetContentLength()).To(BeNumerically("==", 1234))
				Expect(startStopEvent.StartTimestamp).NotTo(Equal(startStopEvent.StopTimestamp))
			})
		})

		Context("if round tripper returns an error", func() {
			It("should emit a stop event with blank response fields", func() {
				fakeRoundTripper.fakeError = errors.New("fakeEmitter error")
				rt.RoundTrip(req)

				Expect(fakeEmitter.GetMessages()[0].Event).To(BeAssignableToTypeOf(new(events.HttpStartStop)))

				startStopEvent := fakeEmitter.GetMessages()[0].Event.(*events.HttpStartStop)
				Expect(startStopEvent.GetStatusCode()).To(BeNumerically("==", 0))
				Expect(startStopEvent.GetContentLength()).To(BeNumerically("==", 0))
			})
		})
	})
})

type FakeRoundTripper struct {
	fakeError error
}

func (frt *FakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 123, ContentLength: 1234}, frt.fakeError
}

type fakeCancelableRoundTripper struct {
	fakeError error
	canceled  bool
}

func (frt *fakeCancelableRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 123, ContentLength: 1234}, frt.fakeError
}

func (frt *fakeCancelableRoundTripper) CancelRequest(req *http.Request) {
	frt.canceled = true
}

type canceler interface {
	CancelRequest(*http.Request)
}
