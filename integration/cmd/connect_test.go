package cmd_test

import (
	"net/http"
	"os"
	"os/exec"

	natsserver "github.com/nats-io/gnatsd/server"
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
		configFile *os.File

		natsPassword   string
		natsServerOpts natsserver.Options
		natsServer     *natsserver.Server
	)

	BeforeEach(func() {
		httpClient = makeTestHttpClient()

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

	Context("when we invoke connect commmand with valid config", func() {
		Context("where server TLS is disabled", func() {
			BeforeEach(func() {
				configFile, err = createOpiConfigFromFixtures(defaultEiriniConfig(natsServerOpts))

				command = exec.Command(cmdPath, "connect", "-c", configFile.Name())
				_, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should start serving HTTP traffic", func() {
				Eventually(func() (*http.Response, error) {
					return httpClient.Get("http://localhost:8085/")
				}, "5s").ShouldNot(BeNil())
			})
		})

		Context("where server TLS is enabled", func() {
			BeforeEach(func() {
				config := defaultEiriniConfig(natsServerOpts)
				config.Properties.ServerCertPath = pathToTestFixture("cert")
				config.Properties.ServerKeyPath = pathToTestFixture("key")
				config.Properties.TLSPort = 4321
				configFile, err = createOpiConfigFromFixtures(config)

				command = exec.Command(cmdPath, "connect", "-c", configFile.Name())
				_, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should start serving HTTPS traffic", func() {
				Eventually(func() (*http.Response, error) {
					return httpClient.Get("https://localhost:4321/")
				}, "5s").Should(PointTo(MatchFields(IgnoreExtras, Fields{
					"TLS": PointTo(MatchFields(IgnoreExtras, Fields{
						"HandshakeComplete": BeTrue(),
					})),
				})))
			})
		})
	})
})
