package cmd_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/nats-server/server"
	natstest "github.com/nats-io/nats-server/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("connect command", func() {
	var (
		err        error
		httpClient *http.Client

		cmdPath    string
		command    *exec.Cmd
		config     *eirini.Config
		configFile *os.File

		natsPassword   string
		natsServerOpts natsserver.Options
		natsServer     *natsserver.Server
	)

	BeforeEach(func() {
		httpClient = makeTestHTTPClient()

		natsPassword = "password"
		natsServerOpts = natstest.DefaultTestOptions
		natsServerOpts.Username = "nats"
		natsServerOpts.Password = natsPassword
		natsServerOpts.Port = int(nextAvailPort())
		natsServer = natstest.RunServer(&natsServerOpts)

		cmdPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		natsServer.Shutdown()

		if command != nil {
			err = command.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("when we invoke connect command with valid config", func() {

		BeforeEach(func() {
			config = defaultEiriniConfig(natsServerOpts)
			config.Properties.ServerCertPath = pathToTestFixture("cert")
			config.Properties.ServerKeyPath = pathToTestFixture("key")
			config.Properties.TLSPort = int(nextAvailPort())
			config.Properties.ClientCAPath = pathToTestFixture("cert")
			configFile, err = createOpiConfigFromFixtures(config)

			command = exec.Command(cmdPath, "connect", "-c", configFile.Name())
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
				Eventually(func() string {
					_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
					if err != nil {
						return err.Error()
					}
					return ""
				}, "5s").Should(ContainSubstring("remote error: tls: bad certificate"))
			})
		})

		Context("when sending a request with an invalid client certificate", func() {
			BeforeEach(func() {
				clientCert, err := tls.LoadX509KeyPair(pathToTestFixture("untrusted-cert"), pathToTestFixture("untrusted-key"))
				Expect(err).ToNot(HaveOccurred())

				httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{clientCert}
			})

			It("we should receive a mTLS-related connection failure", func() {
				Eventually(func() string {
					_, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort))
					if err != nil {
						return err.Error()
					}
					return ""
				}, "5s").Should(ContainSubstring("remote error: tls: bad certificate"))
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
