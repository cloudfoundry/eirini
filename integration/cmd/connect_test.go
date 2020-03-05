package cmd_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"

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

	BeforeEach(func() {
		var err error
		httpClient, err = util.MakeTestHTTPClient()
		Expect(err).ToNot(HaveOccurred())
		configFilePath = ""
		session = nil
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	Context("when we invoke connect command with valid config", func() {
		BeforeEach(func() {
			config = util.DefaultEiriniConfig("test-ns")
			configFile, err := util.CreateOpiConfigFromFixtures(config)
			Expect(err).ToNot(HaveOccurred())
			configFilePath = configFile.Name()

			command := exec.Command(cmdPath, "connect", "-c", configFilePath) // #nosec G204
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() error {
				_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
				return err
			}, "10s").Should(Succeed())

		})

		Context("when sending a request without a client certificate", func() {
			It("should receive a mTLS-related connection failure", func() {
				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{}
				_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
				Expect(err).To(MatchError(ContainSubstring("remote error: tls: bad certificate")))

			})
		})

		Context("when sending a request with an invalid client certificate", func() {
			BeforeEach(func() {
				clientCert, err := tls.LoadX509KeyPair(pathToTestFixture("untrusted-cert"), pathToTestFixture("untrusted-key"))
				Expect(err).ToNot(HaveOccurred())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{clientCert}
			})

			It("we should receive a mTLS-related connection failure", func() {
				_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
				Expect(err).To(MatchError(ContainSubstring("remote error: tls: bad certificate")))
			})
		})

		Context("when sending a request with a valid client certificate", func() {
			It("should successfully connect", func() {
				Expect(httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"TLS": PointTo(MatchFields(IgnoreExtras, Fields{
						"HandshakeComplete": BeTrue(),
					})),
				})))
			})
		})
	})

	Context("when we invoke connect command with invalid config", func() {
		Context("file is missing", func() {
			It("should fail", func() {
				var err error
				command := exec.Command(cmdPath, "connect", "-c", "not-found.yml") // #nosec G204
				session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, "10s").Should(gexec.Exit())
				Expect(session.ExitCode()).NotTo(BeZero())
			})
		})
	})
})
