package cmd_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	natsserver "github.com/nats-io/nats-server/v2/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	fixture    *tests.Fixture
	eiriniBins tests.EiriniBinaries
)

var _ = SynchronizedBeforeSuite(func() []byte {
	eiriniBins = tests.NewEiriniBinaries()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())
	SetDefaultConsistentlyDuration(time.Second * 2)

	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
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
