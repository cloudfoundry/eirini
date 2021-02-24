package cmd_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("connect command", func() {
	var (
		httpClient *http.Client

		session         *gexec.Session
		config          *eirini.APIConfig
		configFilePath  string
		envVarOverrides []string
	)

	makeRequest := func() (*http.Response, error) {
		resp, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/apps", config.TLSPort))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return resp, fmt.Errorf("Errorish status code: %d (%s)", resp.StatusCode, resp.Status)
		}

		return resp, nil
	}

	BeforeEach(func() {
		envVarOverrides = []string{}
		var err error
		httpClient, err = tests.MakeTestHTTPClient()
		Expect(err).ToNot(HaveOccurred())

		configFilePath = ""
		session = nil
		config = tests.DefaultAPIConfig("test-ns", fixture.NextAvailablePort())
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.OPI.Run(config, envVarOverrides...)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	Context("invoke connect command with TLS config", func() {
		It("starts serving", func() {
			Eventually(func() error {
				_, err := makeRequest()

				return err
			}, "10s").Should(Succeed())
		})

		Context("send a request without a client certificate", func() {
			It("receives a mTLS-related connection failure", func() {
				Eventually(func() error {
					_, err := makeRequest()

					return err
				}, "10s").Should(Succeed())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{}
				_, err := makeRequest()
				Expect(err).To(MatchError(ContainSubstring("remote error: tls: bad certificate")))
			})
		})

		Context("send a request with an invalid client certificate", func() {
			var clientCert tls.Certificate

			BeforeEach(func() {
				var err error
				clientCert, err = tls.LoadX509KeyPair(pathToTestFixture("untrusted-cert"), pathToTestFixture("untrusted-key"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns a mTLS-related connection failure", func() {
				Eventually(func() error {
					_, err := makeRequest()

					return err
				}, "10s").Should(Succeed())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{clientCert}

				_, err := makeRequest()
				Expect(err).To(MatchError(
					ContainSubstring("remote error: tls: bad certificate")))
			})
		})

		Context("when sending a request with a valid client certificate", func() {
			It("should successfully connect", func() {
				var resp *http.Response
				var err error
				Eventually(func() error {
					resp, err = makeRequest()

					return err
				}, "10s").Should(Succeed())

				Expect(resp).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"TLS": PointTo(MatchFields(IgnoreExtras, Fields{
						"HandshakeComplete": BeTrue(),
					})),
				})))
			})
		})
	})

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.OPI.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to read config file"))
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

	Context("invoke connect command with a nonexistent config", func() {
		var failingSession *gexec.Session

		BeforeEach(func() {
			failingSession = eiriniBins.OPI.RunWithConfig("foo")
		})

		It("fails", func() {
			Eventually(failingSession, "10s").Should(gexec.Exit())
			Expect(failingSession.ExitCode()).NotTo(BeZero())
			Expect(failingSession.Err).To(gbytes.Say("foo: no such file or directory"))
		})
	})

	Context("invoke connect command with non-existent TLS certs", func() {
		var certDir string

		BeforeEach(func() {
			certDir, _ = tests.GenerateKeyPairDir("tls", "localhost")
			envVarOverrides = []string{fmt.Sprintf("%s=%s", eirini.EnvCCCertDir, certDir)}
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

		When("eirini is configured to serve plaintext", func() {
			BeforeEach(func() {
				config = tests.DefaultAPIConfig("test-ns", fixture.NextAvailablePort())
				config.ServePlaintext = true
				config.PlaintextPort = fixture.NextAvailablePort()

				configFile, err := tests.CreateConfigFile(config)
				Expect(err).ToNot(HaveOccurred())
				configFilePath = configFile.Name()
			})

			It("starts a plaintext http connection", func() {
				plaintextClient := &http.Client{}

				Eventually(func() error {
					_, err := plaintextClient.Get(fmt.Sprintf("http://localhost:%d/", config.PlaintextPort))

					return err
				}, "10s").Should(Succeed())
			})
		})
	})
})
