package main_test

import (
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Domain API", func() {
	BeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
		bbsProcess = ginkgomon.Invoke(bbsRunner)
	})

	Describe("UpsertDomain", func() {
		var existingDomain string

		BeforeEach(func() {
			existingDomain = "existing-domain"
			err := client.UpsertDomain(logger, existingDomain, 100*time.Second)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does emit latency metrics", func() {
			err := client.UpsertDomain(logger, existingDomain, 200*time.Second)
			Expect(err).ToNot(HaveOccurred())

			Eventually(testMetricsChan).Should(Receive(testhelpers.MatchV2Metric(testhelpers.MetricAndValue{
				Name: "RequestLatency",
			})))
		})

		It("emits request counting metrics", func() {
			err := client.UpsertDomain(logger, existingDomain, 200*time.Second)
			Expect(err).ToNot(HaveOccurred())

			var total uint64
			Eventually(testMetricsChan).Should(Receive(
				SatisfyAll(
					WithTransform(func(source *loggregator_v2.Envelope) *loggregator_v2.Counter {
						return source.GetCounter()
					}, Not(BeNil())),
					WithTransform(func(source *loggregator_v2.Envelope) string {
						return source.GetCounter().Name
					}, Equal("RequestCount")),
					WithTransform(func(source *loggregator_v2.Envelope) uint64 {
						total += source.GetCounter().Delta
						return total
					}, BeEquivalentTo(2)),
				),
			))
		})

		It("updates the TTL when updating an existing domain", func() {
			err := client.UpsertDomain(logger, existingDomain, 1*time.Second)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []string {
				domains, err := client.Domains(logger)
				Expect(err).NotTo(HaveOccurred())
				return domains
			}).ShouldNot(ContainElement(existingDomain))
		})

		It("creates a domain with the desired TTL", func() {
			err := client.UpsertDomain(logger, "new-domain", 54*time.Second)
			Expect(err).ToNot(HaveOccurred())
			domains, err := client.Domains(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(domains).To(ContainElement("new-domain"))
		})
	})

	Describe("Domains", func() {
		var expectedDomains []string
		var actualDomains []string
		var getErr error

		BeforeEach(func() {
			expectedDomains = []string{"domain-0", "domain-1"}
			for i, d := range expectedDomains {
				err := client.UpsertDomain(logger, d, time.Second*time.Duration(100*(i+1)))
				Expect(err).NotTo(HaveOccurred())
			}

			actualDomains, getErr = client.Domains(logger)
		})

		It("responds without error", func() {
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("has the correct number of responses", func() {
			Expect(actualDomains).To(HaveLen(2))
		})

		It("has the correct domains from the bbs", func() {
			Expect(expectedDomains).To(ConsistOf(actualDomains))
		})
	})
})
