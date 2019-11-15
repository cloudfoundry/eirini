package cmd_test

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("MetricsCollector", func() {
	var (
		err error

		command    *exec.Cmd
		cmdPath    string
		config     *eirini.MetricsCollectorConfig
		configFile *os.File
	)

	BeforeEach(func() {
		cmdPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/metrics-collector")

	})

	AfterEach(func() {
		if command != nil {
			err = command.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When metrics-collector is executed with valid loggregator config", func() {

		BeforeEach(func() {
			config = metricsCollectorConfig()
			configFile, err = createMetricsCollectorConfigFile(config)
		})

		It("should be able to start properly", func() {
			command = exec.Command(cmdPath, "-c", configFile.Name())
			_, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
