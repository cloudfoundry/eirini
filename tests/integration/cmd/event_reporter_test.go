package cmd_test

import (
	"os"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("EventReporter", func() {
	var (
		config         *eirini.EventReporterConfig
		configFilePath string
		session        *gexec.Session
	)

	BeforeEach(func() {
		config = &eirini.EventReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  "default",
				ConfigPath: fixture.KubeConfigPath,
			},
			CcInternalAPI: "doesitmatter.com",
			CCCertPath:    pathToTestFixture("cert"),
			CCCAPath:      pathToTestFixture("cert"),
			CCKeyPath:     pathToTestFixture("key"),
		}
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.EventsReporter.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	When("event reporter is executed with a valid config", func() {
		It("should be able to start properly", func() {
			Consistently(session, "5s").ShouldNot(gexec.Exit())
		})
	})

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.EventsReporter.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to read file"))
		})
	})

	When("config is missing kubeconfig path", func() {
		BeforeEach(func() {
			config.ConfigPath = ""
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("invalid configuration: no configuration has been provided"))
		})
	})

	When("event reporter command with non-existent TLS certs", func() {
		BeforeEach(func() {
			config.CCCAPath = "/does/not/exist"
			config.CCCertPath = "/does/not/exist"
			config.CCKeyPath = "/does/not/exist"
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to load keypair: open /does/not/exist: no such file or directory"))
		})
	})
})
