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
				ConfigPath: pathToTestFixture("kube.conf"),
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
			Expect(session.Err).To(gbytes.Say("failed to read file"))
		})
	})

	When("the config file is not valid yaml", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.StagingReporter.Restart(pathToTestFixture("invalid.yml"), session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to unmarshal yaml"))
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

	When("cert paths do not exist", func() {
		BeforeEach(func() {
			config.EiriniCertPath = ""
			config.EiriniKeyPath = ""
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to load keypair: open : no such file or directory"))
		})
	})
})
