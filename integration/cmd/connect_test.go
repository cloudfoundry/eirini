package cmd_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	getRootError := func() error {
		_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
		return err
	}

	getRoot := func() *http.Response {
		resp, _ := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
		return resp
	}

	BeforeEach(func() {
		var err error
		httpClient, err = util.MakeTestHTTPClient()
		Expect(err).ToNot(HaveOccurred())

		configFilePath = ""
		session = nil
	})

	JustBeforeEach(func() {
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
		BeforeEach(func() {
			config = util.DefaultEiriniConfig("test-ns", fixture.NextAvailablePort())
			configFile, err := util.CreateConfigFile(config)
			Expect(err).ToNot(HaveOccurred())
			configFilePath = configFile.Name()
		})

		It("starts serving", func() {
			Eventually(getRootError, "10s").Should(Succeed())
		})

		Context("send a request without a client certificate", func() {
			It("receives a mTLS-related connection failure", func() {
				Eventually(getRootError, "10s").Should(Succeed())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{}
				Expect(getRootError()).To(MatchError(
					ContainSubstring("remote error: tls: bad certificate")))
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
				Eventually(getRootError, "10s").Should(Succeed())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{clientCert}

				Expect(getRootError()).To(MatchError(
					ContainSubstring("remote error: tls: bad certificate")))
			})
		})

		Context("when sending a request with a valid client certificate", func() {
			It("should successfully connect", func() {
				Eventually(getRootError, "10s").Should(Succeed())

				Expect(getRoot()).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"TLS": PointTo(MatchFields(IgnoreExtras, Fields{
						"HandshakeComplete": BeTrue(),
					})),
				})))
			})
		})
	})

	Context("invoke connect command with non-existent config", func() {
		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
		})
	})

	Context("invoke connect command without TLS config", func() {

		BeforeEach(func() {
			config = util.DefaultEiriniConfig("test-ns", fixture.NextAvailablePort())
			config.Properties.ClientCAPath = ""
			config.Properties.ServerCertPath = ""
			config.Properties.ServerKeyPath = ""

			configFile, err := util.CreateConfigFile(config)
			Expect(err).ToNot(HaveOccurred())
			configFilePath = configFile.Name()
		})

		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())

		})

		Context("eirini is configured to serve plaintext", func() {
			BeforeEach(func() {
				config = util.DefaultEiriniConfig("test-ns", fixture.NextAvailablePort())
				config.Properties.ClientCAPath = ""
				config.Properties.ServerCertPath = ""
				config.Properties.ServerKeyPath = ""
				config.Properties.ServePlaintext = true
				config.Properties.PlaintextPort = fixture.NextAvailablePort()

				configFile, err := util.CreateConfigFile(config)
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
