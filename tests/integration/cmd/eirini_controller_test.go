package cmd_test

import (
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("EiriniController", func() {
	var (
		config         *eirini.Config
		configFilePath string
		session        *gexec.Session
	)

	BeforeEach(func() {
		config = tests.DefaultEiriniConfig(fixture.Namespace, fixture.NextAvailablePort())
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.EiriniController.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	It("should be able to start properly", func() {
		Consistently(session).ShouldNot(gexec.Exit())
	})

	When("namespace is not configured", func() {
		BeforeEach(func() {
			config.Properties.Namespace = ""
		})

		It("panics", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("must set namespace"))
		})
	})

	When("listening on multiple namespaces", func() {
		BeforeEach(func() {
			config.Properties.EnableMultiNamespaceSupport = true
			config.Properties.Namespace = ""
		})

		It("starts ok", func() {
			Consistently(session).ShouldNot(gexec.Exit())
		})
	})
})
