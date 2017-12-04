package main_test

import (
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"github.com/hashicorp/consul/api"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceRegistration", func() {
	Context("when the bbs service starts", func() {
		Context("when consul is enabled", func() {
			BeforeEach(func() {
				bbsConfig.EnableConsulServiceRegistration = true
				bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
				bbsProcess = ginkgomon.Invoke(bbsRunner)
			})

			It("registers itself with consul", func() {
				client := consulRunner.NewClient()
				services, err := client.Agent().Services()
				Expect(err).ToNot(HaveOccurred())

				Expect(services).To(HaveKeyWithValue("bbs",
					&api.AgentService{
						Service: "bbs",
						ID:      "bbs",
						Port:    bbsPort,
					}))
			})

			It("registers a TTL healthcheck", func() {
				client := consulRunner.NewClient()
				checks, err := client.Agent().Checks()
				Expect(err).ToNot(HaveOccurred())

				Expect(checks).To(HaveKeyWithValue("service:bbs",
					&api.AgentCheck{
						Node:        "0",
						CheckID:     "service:bbs",
						Name:        "Service 'bbs' check",
						Status:      "passing",
						ServiceID:   "bbs",
						ServiceName: "bbs",
					}))
			})
		})

		Context("when consul is disabled", func() {
			BeforeEach(func() {
				bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
				bbsProcess = ginkgomon.Invoke(bbsRunner)
			})

			It("does not register itself with consul", func() {
				client := consulRunner.NewClient()
				services, err := client.Agent().Services()
				Expect(err).ToNot(HaveOccurred())

				Expect(services).NotTo(HaveKey("bbs"))
			})
		})
	})
})
