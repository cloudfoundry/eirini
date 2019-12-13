package cmd_test

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("RouteCollector", func() {
	var (
		err error

		command    *exec.Cmd
		cmdPath    string
		config     *eirini.RouteEmitterConfig
		configFile *os.File

		natsPassword   string
		natsServerOpts natsserver.Options
		natsServer     *natsserver.Server
	)

	BeforeEach(func() {

		natsPassword = "password"
		natsServerOpts = natstest.DefaultTestOptions
		natsServerOpts.Username = "nats"
		natsServerOpts.Password = natsPassword
		natsServerOpts.Port = 61000 + ginkgoconfig.GinkgoConfig.ParallelNode
		natsServer = natstest.RunServer(&natsServerOpts)

		cmdPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/route-collector")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		natsServer.Shutdown()

		if command != nil {
			err = command.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When route collector is executed with valid nats config", func() {
		BeforeEach(func() {
			config = defaultRouteEmitterConfig(natsServerOpts)
			configFile, err = createRouteEmitterConfig(config)
		})

		It("should be able to start properly", func() {
			command = exec.Command(cmdPath, "-c", configFile.Name())
			_, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
