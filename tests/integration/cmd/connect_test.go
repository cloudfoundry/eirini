package cmd_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

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

		session        *gexec.Session
		config         *eirini.Config
		configFilePath string
	)

	makeRequest := func() (*http.Response, error) {
		resp, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/apps", config.Properties.TLSPort))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return resp, fmt.Errorf("Errorish status code: %d (%s)", resp.StatusCode, resp.Status)
		}

		return resp, nil
	}

	BeforeEach(func() {
		var err error
		httpClient, err = tests.MakeTestHTTPClient()
		Expect(err).ToNot(HaveOccurred())

		configFilePath = ""
		session = nil
		config = tests.DefaultEiriniConfig("test-ns", fixture.NextAvailablePort())
	})

	JustBeforeEach(func() {
		configFile, err := tests.CreateConfigFile(config)
		Expect(err).ToNot(HaveOccurred())
		configFilePath = configFile.Name()
		session, configFilePath = eiriniBins.OPI.Run(config)
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

	When("config is missing kubeconfig path", func() {
		BeforeEach(func() {
			config.Properties.ConfigPath = ""
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("invalid configuration: no configuration has been provided"))
		})
	})

	Context("invoke connect command with an empty config", func() {
		BeforeEach(func() {
			config = nil
		})

		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("invalid configuration: no configuration has been provided"))
		})
	})

	Context("invoke connect command with non-existent TLS certs", func() {
		When("the cc CA file is missing", func() {
			BeforeEach(func() {
				config.Properties.CCCAPath = "/somewhere/over/the/rainbow"
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"CC CA" file at "/somewhere/over/the/rainbow" does not exist`))
			})
		})

		When("the cc cert file is missing", func() {
			BeforeEach(func() {
				config.Properties.CCCertPath = "/somewhere/over/the/rainbow"
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"CC Cert" file at "/somewhere/over/the/rainbow" does not exist`))
			})
		})

		When("the cc key file is missing", func() {
			BeforeEach(func() {
				config.Properties.CCKeyPath = "/somewhere/over/the/rainbow"
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).Should(gbytes.Say(`"CC Key" file at "/somewhere/over/the/rainbow" does not exist`))
			})
		})

		When("eirini is configured to serve plaintext", func() {
			BeforeEach(func() {
				config = tests.DefaultEiriniConfig("test-ns", fixture.NextAvailablePort())
				config.Properties.ServePlaintext = true
				config.Properties.PlaintextPort = fixture.NextAvailablePort()

				configFile, err := tests.CreateConfigFile(config)
				Expect(err).ToNot(HaveOccurred())
				configFilePath = configFile.Name()
			})

			It("starts a plaintext http connection", func() {
				plaintextClient := &http.Client{}

				Eventually(func() error {
					_, err := plaintextClient.Get(fmt.Sprintf("http://localhost:%d/", config.Properties.PlaintextPort))

					return err
				}, "10s").Should(Succeed())
			})
		})
	})
})
