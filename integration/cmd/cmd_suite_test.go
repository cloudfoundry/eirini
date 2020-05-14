package cmd_test

import (
	"io/ioutil"
	"os"
	"testing"

	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/nats-server/v2/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	yaml "gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	cmdPath string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	path, err := gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
	Expect(err).NotTo(HaveOccurred())
	return []byte(path)
}, func(data []byte) {
	cmdPath = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cmd Suite")
}

func pathToTestFixture(relativePath string) string {
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	return cwd + "/../fixtures/" + relativePath
}

func defaultRouteEmitterConfig(natsServerOpts natsserver.Options) *eirini.RouteEmitterConfig {
	config := &eirini.RouteEmitterConfig{
		KubeConfig: eirini.KubeConfig{
			ConfigPath: pathToTestFixture("kube.conf"),
		},
		NatsIP:       natsServerOpts.Host,
		NatsPort:     natsServerOpts.Port,
		NatsPassword: natsServerOpts.Password,
	}

	return config
}

func metricsCollectorConfig() *eirini.MetricsCollectorConfig {
	config := &eirini.MetricsCollectorConfig{
		KubeConfig: eirini.KubeConfig{
			ConfigPath: pathToTestFixture("kube.conf"),
		},
		LoggregatorCAPath:   pathToTestFixture("cert"),
		LoggregatorCertPath: pathToTestFixture("cert"),
		LoggregatorKeyPath:  pathToTestFixture("key"),
	}

	return config
}

func defaultEventReporterConfig() *eirini.EventReporterConfig {
	config := &eirini.EventReporterConfig{
		KubeConfig: eirini.KubeConfig{
			Namespace:  "default",
			ConfigPath: pathToTestFixture("kube.conf"),
		},
		CcInternalAPI: "doesitmatter.com",
		CCCertPath:    pathToTestFixture("cert"),
		CCCAPath:      pathToTestFixture("cert"),
		CCKeyPath:     pathToTestFixture("key"),
	}

	return config
}

func defaultReporterConfig() *eirini.ReporterConfig {
	config := &eirini.ReporterConfig{
		KubeConfig: eirini.KubeConfig{
			ConfigPath: pathToTestFixture("kube.conf"),
		},
		EiriniCertPath: pathToTestFixture("cert"),
		EiriniKeyPath:  pathToTestFixture("key"),
		CAPath:         pathToTestFixture("cert"),
	}

	return config
}

func createRouteEmitterConfig(config *eirini.RouteEmitterConfig) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	return createConfigFile(bs)
}

func createMetricsCollectorConfigFile(config *eirini.MetricsCollectorConfig) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	return createConfigFile(bs)
}

func createEventReporterConfigFile(config *eirini.EventReporterConfig) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	return createConfigFile(bs)
}

func createStagingReporterConfigFile(config *eirini.ReporterConfig) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	return createConfigFile(bs)
}

func createConfigFile(yamlBytes []byte) (*os.File, error) {
	configFile, err := ioutil.TempFile("", "config.yml")
	Expect(err).ToNot(HaveOccurred())

	err = ioutil.WriteFile(configFile.Name(), yamlBytes, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return configFile, err
}
