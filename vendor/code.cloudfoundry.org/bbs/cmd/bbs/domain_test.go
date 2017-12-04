package main_test

import (
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	events "github.com/cloudfoundry/sonde-go/events"
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

			var sawRequestLatency bool
			timeout := time.After(50 * time.Millisecond)
		OUTER_LOOP:
			for {
				select {
				case envelope := <-testMetricsChan:
					if envelope.GetEventType() == events.Envelope_ValueMetric {
						if *envelope.ValueMetric.Name == "RequestLatency" {
							sawRequestLatency = true
						}
					}
				case <-timeout:
					break OUTER_LOOP
				}
			}
			Expect(sawRequestLatency).To(BeTrue())
		})

		It("emits request counting metrics", func() {
			err := client.UpsertDomain(logger, existingDomain, 200*time.Second)
			Expect(err).ToNot(HaveOccurred())

			timeout := time.After(50 * time.Millisecond)
			var total uint64
		OUTER_LOOP:
			for {
				select {
				case envelope := <-testMetricsChan:
					By("receive event")
					if envelope.GetEventType() == events.Envelope_CounterEvent {
						counter := envelope.CounterEvent
						if *counter.Name == "RequestCount" {
							total += *counter.Delta
						}
					}
				case <-timeout:
					break OUTER_LOOP
				}
			}

			Expect(total).To(BeEquivalentTo(2))
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
