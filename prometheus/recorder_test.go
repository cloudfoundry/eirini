package prometheus_test

import (
	"fmt"

	"code.cloudfoundry.org/eirini/prometheus"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var _ = Describe("Recorder", func() {
	var (
		recorder prometheus.Recorder
		logger   *lagertest.TestLogger
		registry metrics.RegistererGatherer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("prometheus-recorder")
		registry = api.NewRegistry()

		var err error
		recorder, err = prometheus.NewRecorder(logger, registry)
		Expect(err).NotTo(HaveOccurred())
	})

	It("records", func() {
		recorder.Increment(prometheus.LRPCreations)
		Expect(getCounter(registry, prometheus.LRPCreations)).To(Equal(1))
	})

	When("the counter does not exist", func() {
		It("prints a warning log", func() {
			recorder.Increment("does-not-exist")
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			Expect(logs[0].LogLevel).To(Equal(lager.ERROR))
			Expect(logs[0].Message).To(ContainSubstring("unknown-counter"))
			Expect(logs[0].Data).To(HaveKeyWithValue("counter-name", "does-not-exist"))
		})
	})

	When("using a shared registry", func() {
		var otherRecorder prometheus.Recorder

		BeforeEach(func() {
			var err error
			otherRecorder, err = prometheus.NewRecorder(logger, registry)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adopts the existing counters", func() {
			recorder.Increment(prometheus.LRPCreations)
			otherRecorder.Increment(prometheus.LRPCreations)

			Expect(getCounter(registry, prometheus.LRPCreations)).To(Equal(2))
		})
	})
})

func getCounter(registry api.Gatherer, counterName string) int {
	metricFamilies, err := registry.Gather()
	Expect(err).NotTo(HaveOccurred())

	for _, mf := range metricFamilies {
		if *mf.Name != counterName {
			continue
		}

		for _, m := range mf.Metric {
			if m.Counter == nil {
				Fail(fmt.Sprintf("Metric %q has no counter set", counterName))

				return -1
			}

			return int(*m.Counter.Value)
		}
	}

	Fail(fmt.Sprintf("Could not find counter %q", counterName))

	return -1
}
