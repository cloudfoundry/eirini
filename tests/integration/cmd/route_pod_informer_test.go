package cmd_test

import (
	"os"

	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("RoutePodInformer", func() {
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

		config = defaultRouteEmitterConfig(natsServerOpts)
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.RoutePodInformer.Run(config)
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

	When("route collector is executed with valid config", func() {
		It("should be able to start properly", func() {
			Consistently(session, "5s").ShouldNot(gexec.Exit())
		})
	})

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.RoutePodInformer.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to read config from /does/not/exist: failed to read file"))
		})
	})

	When("the config file is not valid yaml", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.RoutePodInformer.Restart(pathToTestFixture("invalid.yml"), session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to read config from .*/invalid.yml: failed to unmarshal yaml"))
		})
	})

	When("config is missing kubeconfig path", func() {
		BeforeEach(func() {
			config.ConfigPath = ""
		})

		It("fails", func() {
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("Failed to get kubeconfig: invalid configuration: no configuration has been provided"))
		})
	})

	When("EnableMultiNamespaceSupport is false", func() {
		BeforeEach(func() {
			config.EnableMultiNamespaceSupport = false
			config.Namespace = fixture.Namespace
		})

		It("should be able to start properly", func() {
			Consistently(session, "5s").ShouldNot(gexec.Exit())
		})

		When("the namespace is not set", func() {
			BeforeEach(func() {
				config.Namespace = ""
			})

			It("should exit with a useful error message", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).To(gbytes.Say("must set namespace in config when enableMultiNamespaceSupport is not set"))
			})
		})
	})
})
