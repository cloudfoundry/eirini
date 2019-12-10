package cmd_test

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"

	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/nats-server/v2/server"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	yaml "gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	lastPortUsed int
	portLock     sync.Mutex
	once         sync.Once
	cmdPath      string
)

var _ = BeforeSuite(func() {
	var err error
	cmdPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cmd Suite")
}

func pathToTestFixture(relativePath string) string {
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	return cwd + "/fixtures/" + relativePath
}

func defaultEiriniConfig() *eirini.Config {
	config := &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: pathToTestFixture("kube.conf"),
			},
			CCCAPath:   pathToTestFixture("cert"),
			CCCertPath: pathToTestFixture("cert"),
			CCKeyPath:  pathToTestFixture("key"),
		},
	}

	return config
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

func createOpiConfigFromFixtures(config *eirini.Config) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	return createConfigFile(bs)
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

func createConfigFile(yamlBytes []byte) (*os.File, error) {
	configFile, err := ioutil.TempFile("", "config.yml")
	Expect(err).ToNot(HaveOccurred())

	err = ioutil.WriteFile(configFile.Name(), yamlBytes, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return configFile, err
}

func makeTestHTTPClient() *http.Client {
	bs, err := ioutil.ReadFile(pathToTestFixture("cert"))
	Expect(err).ToNot(HaveOccurred())

	certPool := x509.NewCertPool()
	Expect(certPool.AppendCertsFromPEM(bs)).To(BeTrue())
	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}
	httpClient := cfhttp.NewClient(cfhttp.WithTLSConfig(tlsConfig))
	return httpClient
}

func nextAvailPort() uint16 {
	portLock.Lock()
	defer portLock.Unlock()

	if lastPortUsed == 0 {
		once.Do(func() {
			const portRangeStart = 61000
			lastPortUsed = portRangeStart + ginkgoconfig.GinkgoConfig.ParallelNode
		})
	}

	lastPortUsed += ginkgoconfig.GinkgoConfig.ParallelTotal
	return uint16(lastPortUsed)
}
