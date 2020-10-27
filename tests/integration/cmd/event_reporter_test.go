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
				ConfigPath:                  fixture.KubeConfigPath,
				EnableMultiNamespaceSupport: true,
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
			Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to read file"))
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

	When("the cc CA file is missing", func() {
		BeforeEach(func() {
			config.CCCAPath = "/somewhere/over/the/rainbow"
		})

		It("should exit with a useful error message", func() {
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).Should(gbytes.Say(`"CC CA" file at "/somewhere/over/the/rainbow" does not exist`))
		})
	})

	When("the cc cert file is missing", func() {
		BeforeEach(func() {
			config.CCCertPath = "/somewhere/over/the/rainbow"
		})

		It("should exit with a useful error message", func() {
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).Should(gbytes.Say(`"CC Cert" file at "/somewhere/over/the/rainbow" does not exist`))
		})
	})

	When("the cc key file is missing", func() {
		BeforeEach(func() {
			config.CCKeyPath = "/somewhere/over/the/rainbow"
		})

		It("should exit with a useful error message", func() {
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).Should(gbytes.Say(`"CC Key" file at "/somewhere/over/the/rainbow" does not exist`))
		})
	})

	When("EnableMultiNamespaceSupport is false", func() {
		BeforeEach(func() {
			config.EnableMultiNamespaceSupport = false
			config.Namespace = fixture.Namespace
		})

		It("should be able to start properly", func() {
			Consistently(session, "5s").ShouldNot(gexec.Exit())
		})

		When("the namespace is not set", func() {
			BeforeEach(func() {
				config.Namespace = ""
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).To(gbytes.Say("must set namespace in config when enableMultiNamespaceSupport is not set"))
			})
		})
	})
})
