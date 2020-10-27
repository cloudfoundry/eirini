package cmd_test

import (
	"os"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("StagingReporter", func() {
	var (
		config         *eirini.StagingReporterConfig
		configFilePath string
		session        *gexec.Session
	)

	BeforeEach(func() {
		config = &eirini.StagingReporterConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath:                  pathToTestFixture("kube.conf"),
				EnableMultiNamespaceSupport: true,
			},
			EiriniCertPath: pathToTestFixture("cert"),
			EiriniKeyPath:  pathToTestFixture("key"),
			CAPath:         pathToTestFixture("cert"),
		}
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.StagingReporter.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	When("staging reporter is executed with a valid config", func() {
		It("should be able to start properly", func() {
			Consistently(session, "5s").ShouldNot(gexec.Exit())
		})
	})

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.StagingReporter.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to read file"))
		})
	})

	When("the config file is not valid yaml", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.StagingReporter.Restart(pathToTestFixture("invalid.yml"), session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to unmarshal yaml"))
		})
	})

	When("config is missing kubeconfig path", func() {
		BeforeEach(func() {
			config.ConfigPath = ""
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to get kubeconfig: invalid configuration: no configuration has been provided"))
		})
	})

	When("the Eirini cert file is missing", func() {
		BeforeEach(func() {
			config.EiriniCertPath = "/somewhere/over/the/rainbow"
		})

		It("should exit with a useful error message", func() {
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).Should(gbytes.Say(`"Eirini Cert" file at "/somewhere/over/the/rainbow" does not exist`))
		})
	})

	When("the Eirini key file is missing", func() {
		BeforeEach(func() {
			config.EiriniKeyPath = "/somewhere/over/the/rainbow"
		})

		It("should exit with a useful error message", func() {
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).Should(gbytes.Say(`"Eirini Key" file at "/somewhere/over/the/rainbow" does not exist`))
		})
	})

	When("the Eirini CA file is missing", func() {
		BeforeEach(func() {
			config.CAPath = "/somewhere/over/the/rainbow"
		})

		It("should exit with a useful error message", func() {
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).Should(gbytes.Say(`"Eirini CA" file at "/somewhere/over/the/rainbow" does not exist`))
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
