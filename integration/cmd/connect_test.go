package cmd_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("connect command", func() {
	var (
		httpClient *http.Client

		command    *exec.Cmd
		config     *eirini.Config
		configFile *os.File
	)

	BeforeEach(func() {
		httpClient = makeTestHTTPClient()

	})

	AfterEach(func() {

		if command != nil {
			err := command.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("when we invoke connect command with valid config", func() {

		BeforeEach(func() {
			var err error
			config = defaultEiriniConfig()
			config.Properties.ServerCertPath = pathToTestFixture("cert")
			config.Properties.ServerKeyPath = pathToTestFixture("key")
			config.Properties.TLSPort = int(nextAvailPort())
			config.Properties.ClientCAPath = pathToTestFixture("cert")
			configFile, err = createOpiConfigFromFixtures(config)
			Expect(err).ToNot(HaveOccurred())

			command = exec.Command(cmdPath, "connect", "-c", configFile.Name()) // #nosec G204
			_, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not serve HTTP traffic", func() {
			Eventually(func() error {
				_, err := httpClient.Get("http://localhost:8085/")
				return err
			}, "5s").Should(MatchError(ContainSubstring("connection refused")))
		})

		Context("when sending a request without a client certificate", func() {
			It("we should receive a mTLS-related connection failure", func() {
				Eventually(func() error {
					_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
					return err
				}, "5s").Should(MatchError("remote error: tls: bad certificate"))
			})
		})

		Context("when sending a request with an invalid client certificate", func() {
			BeforeEach(func() {
				clientCert, err := tls.LoadX509KeyPair(pathToTestFixture("untrusted-cert"), pathToTestFixture("untrusted-key"))
				Expect(err).ToNot(HaveOccurred())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{clientCert}
			})

			It("we should receive a mTLS-related connection failure", func() {
				Eventually(func() error {
					_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
					return err
				}, "5s").Should(MatchError("remote error: tls: bad certificate"))
			})
		})

		Context("when sending a request with a valid client certificate", func() {
			BeforeEach(func() {
				clientCert, err := tls.LoadX509KeyPair(pathToTestFixture("cert"), pathToTestFixture("key"))
				Expect(err).ToNot(HaveOccurred())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{clientCert}
			})

			It("we should successfully connect", func() {
				Eventually(func() (*http.Response, error) {
					return httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
				}, "5s").Should(PointTo(MatchFields(IgnoreExtras, Fields{
					"TLS": PointTo(MatchFields(IgnoreExtras, Fields{
						"HandshakeComplete": BeTrue(),
					})),
				})))
			})
		})
	})
})
