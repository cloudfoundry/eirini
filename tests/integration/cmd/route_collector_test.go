package cmd_test

import (
	"os"
	"syscall"

	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("RouteCollector", func() {
	var (
		config         *eirini.RouteEmitterConfig
		configFilePath string
		session        *gexec.Session

		natsPassword   string
		natsServerOpts natsserver.Options
		natsServer     *natsserver.Server
	)

	BeforeEach(func() {
		natsPassword = "password"
		natsServerOpts = natstest.DefaultTestOptions
		natsServerOpts.Username = "nats"
		natsServerOpts.Password = natsPassword
		natsServerOpts.Port = fixture.NextAvailablePort()
		natsServer = natstest.RunServer(&natsServerOpts)
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.RouteCollector.Run(config)
	})

	AfterEach(func() {
		natsServer.Shutdown()

		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	Context("When route collector is executed with valid nats config", func() {
		BeforeEach(func() {
			config = defaultRouteEmitterConfig(natsServerOpts)
		})

		It("should be able to start properly", func() {
			Expect(session.Command.Process.Signal(syscall.Signal(0))).To(Succeed())
		})
	})
})
