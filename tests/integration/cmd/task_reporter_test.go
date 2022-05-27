package cmd_test

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("TaskReporter", func() {
	var (
		config          *eirini.TaskReporterConfig
		configFilePath  string
		session         *gexec.Session
		envVarOverrides []string
	)

	BeforeEach(func() {
		envVarOverrides = []string{}
		config = &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
			CCTLSDisabled:           false,
			LeaderElectionID:        fmt.Sprintf("test-task-reporter-%d", GinkgoParallelProcess()),
			LeaderElectionNamespace: fixture.Namespace,
		}
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.TaskReporter.Run(config, envVarOverrides...)
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

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.TaskReporter.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to read file"))
		})
	})

	When("nonexistent kubeconfig path is provided", func() {
		BeforeEach(func() {
			config.ConfigPath = "foo"
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("foo: no such file or directory"))
		})
	})

	Context("invoke connect command with non-existent TLS certs", func() {
		var certDir string

		BeforeEach(func() {
			certDir, _ = tests.GenerateKeyPairDir("tls", "localhost")
			envVarOverrides = []string{
				fmt.Sprintf("%s=%s", eirini.EnvCCCertDir, certDir),
			}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(certDir)).To(Succeed())
		})

		When("the cc CA file is missing", func() {
			BeforeEach(func() {
				caPath := filepath.Join(certDir, "tls.ca")
				Expect(os.RemoveAll(caPath)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"Cloud Controller CA" file at ".*" does not exist`))
			})
		})

		When("the cc cert file is missing", func() {
			BeforeEach(func() {
				crtPath := filepath.Join(certDir, "tls.crt")
				Expect(os.RemoveAll(crtPath)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"Cloud Controller Cert" file at ".*" does not exist`))
			})
		})

		When("the cc key file is missing", func() {
			BeforeEach(func() {
				keyPath := filepath.Join(certDir, "tls.key")
				Expect(os.RemoveAll(keyPath)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"Cloud Controller Key" file at ".*" does not exist`))
			})
		})
	})
})
