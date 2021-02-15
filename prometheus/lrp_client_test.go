package prometheus_test

import (
	"errors"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/prometheus"
	"code.cloudfoundry.org/eirini/prometheus/prometheusfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRP Client Prometheus Decorator", func() {
	var (
		metricsRecorder *prometheusfakes.FakeRecorder
		lrpClient       *prometheusfakes.FakeLRPClient
		decorator       prometheus.LRPClient
		desireOpts      []shared.Option
		lrp             *opi.LRP
		desireErr       error
	)

	BeforeEach(func() {
		lrpClient = new(prometheusfakes.FakeLRPClient)
		metricsRecorder = new(prometheusfakes.FakeRecorder)
		desireOption := func(resource interface{}) error {
			return nil
		}
		desireOpts = []shared.Option{desireOption}
		lrp = &opi.LRP{}
		decorator = prometheus.NewLRPClientDecorator(lrpClient, metricsRecorder)
	})

	JustBeforeEach(func() {
		desireErr = decorator.Desire("the-namespace", lrp, desireOpts...)
	})

	It("succeeds", func() {
		Expect(desireErr).NotTo(HaveOccurred())
	})

	It("delegates to the LRP client on Desire", func() {
		Expect(lrpClient.DesireCallCount()).To(Equal(1))
		actualNamespace, actualLrp, actualOption := lrpClient.DesireArgsForCall(0)
		Expect(actualNamespace).To(Equal("the-namespace"))
		Expect(actualLrp).To(Equal(lrp))
		Expect(actualOption).To(Equal(desireOpts))
	})

	It("increments the LRP creation counter", func() {
		Expect(metricsRecorder.IncrementCallCount()).To(Equal(1))
		actualCounter := metricsRecorder.IncrementArgsForCall(0)
		Expect(actualCounter).To(Equal(prometheus.LRPCreations))
	})

	When("desiring the lrp fails", func() {
		BeforeEach(func() {
			lrpClient.DesireReturns(errors.New("foo"))
		})

		It("fails", func() {
			Expect(desireErr).To(MatchError("foo"))
		})

		It("does not increment the LRP creation counter", func() {
			Expect(metricsRecorder.IncrementCallCount()).To(Equal(0))
		})
	})
})
