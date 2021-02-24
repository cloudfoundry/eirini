package cmd_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("MetricsCollector", func() {
	var (
		config          *eirini.MetricsCollectorConfig
		configFilePath  string
		session         *gexec.Session
		envVarOverrides []string
	)
	BeforeEach(func() {
		envVarOverrides = []string{}
		config = &eirini.MetricsCollectorConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: pathToTestFixture("kube.conf"),
			},
		}
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.MetricsCollector.Run(config, envVarOverrides...)
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
		Consistently(session, "5s").ShouldNot(gexec.Exit())
	})

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.MetricsCollector.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to read file"))
		})
	})

	When("the config file is not valid yaml", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.MetricsCollector.Restart(pathToTestFixture("invalid.yml"), session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to unmarshal yaml"))
		})
	})
	Context("invoke connect command with non-existent TLS certs", func() {
		var certDir string

		BeforeEach(func() {
			certDir, _ = tests.GenerateKeyPairDir("tls", "localhost")
			envVarOverrides = []string{fmt.Sprintf("%s=%s", eirini.EnvLoggregatorCertDir, certDir)}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(certDir)).To(Succeed())
		})

		When("the loggregator CA file is missing", func() {
			BeforeEach(func() {
				caPath := filepath.Join(certDir, "tls.ca")
				Expect(os.RemoveAll(caPath)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"Loggregator CA" file at ".*" does not exist`))
			})
		})

		When("the loggregator cert file is missing", func() {
			BeforeEach(func() {
				crtPath := filepath.Join(certDir, "tls.crt")
				Expect(os.RemoveAll(crtPath)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"Loggregator Cert" file at ".*" does not exist`))
			})
		})

		When("the loggregator key file is missing", func() {
			BeforeEach(func() {
				keyPath := filepath.Join(certDir, "tls.key")
				Expect(os.RemoveAll(keyPath)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"Loggregator Key" file at ".*" does not exist`))
			})
		})

		When("the loggregator CA file is invalid", func() {
			BeforeEach(func() {
				caPath := filepath.Join(certDir, "tls.ca")
				Expect(ioutil.WriteFile(caPath, []byte("I'm not a cert"), 0600)).To(Succeed())
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`Failed to create loggregator tls config: cannot parse ca cert`))
			})
		})
	})
})
