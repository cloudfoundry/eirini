package cmd_test

import (
	"os"
	"syscall"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("TaskReporter", func() {
	var (
		config         *eirini.TaskReporterConfig
		configFilePath string
		session        *gexec.Session
	)

	BeforeEach(func() {
		config = &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:                   "default",
				EnableMultiNamespaceSupport: false,
				ConfigPath:                  fixture.KubeConfigPath,
			},
			CCTLSDisabled: true,
		}
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.TaskReporter.Run(config)
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
		Consistently(func() error {
			return session.Command.Process.Signal(syscall.Signal(0))
		}).Should(Succeed())
	})

	When("namespace is not configured", func() {
		BeforeEach(func() {
			config.Namespace = ""
		})

		It("panics", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("must set namespace"))
		})
	})

	When("listening on multiple namespaces", func() {
		BeforeEach(func() {
			config.EnableMultiNamespaceSupport = true
			config.Namespace = ""
		})

		It("starts ok", func() {
			Consistently(func() error {
				return session.Command.Process.Signal(syscall.Signal(0))
			}).Should(Succeed())
		})
	})

})
