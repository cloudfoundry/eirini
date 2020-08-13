package cmd_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	natsserver "github.com/nats-io/nats-server/v2/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	fixture    *util.Fixture
	eiriniBins util.EiriniBinaries
	binsPath   string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = util.NewEiriniBinaries(binsPath)

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = util.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
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

func defaultReporterConfig() *eirini.TaskReporterConfig {
	config := &eirini.TaskReporterConfig{
		KubeConfig: eirini.KubeConfig{
			ConfigPath: pathToTestFixture("kube.conf"),
		},
		CCCertPath: pathToTestFixture("cert"),
		CCKeyPath:  pathToTestFixture("key"),
		CAPath:     pathToTestFixture("cert"),
	}

	return config
}

func defaultInstanceIndexEnvInjectorConfig() *eirini.InstanceIndexEnvInjectorConfig {
	config := &eirini.InstanceIndexEnvInjectorConfig{
		KubeConfig: eirini.KubeConfig{
			ConfigPath: pathToTestFixture("kube.conf"),
		},
		ServiceName:                "foo",
		ServiceNamespace:           "default",
		ServicePort:                8080,
		EiriniXOperatorFingerprint: "cmd-test",
	}

	return config
}
