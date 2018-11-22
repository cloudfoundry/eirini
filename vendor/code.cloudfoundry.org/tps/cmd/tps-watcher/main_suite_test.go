package main_test

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/tps/cmd/tpsrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	tpsconfig "code.cloudfoundry.org/tps/config"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"testing"
	"time"
)

var (
	consulRunner *consulrunner.ClusterRunner

	watcher           ifrit.Process
	runner            *ginkgomon.Runner
	disableStartCheck bool

	watcherPath   string
	locketBinPath string

	watcherConfig tpsconfig.WatcherConfig

	fakeCC  *ghttp.Server
	fakeBBS *ghttp.Server
	logger  *lagertest.TestLogger
)

func TestTPS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TPS-Watcher Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	tps, err := gexec.Build("code.cloudfoundry.org/tps/cmd/tps-watcher", "-race")
	Expect(err).NotTo(HaveOccurred())

	locketPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"watcher": tps,
		"locket":  locketPath,
	})
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	binaries := map[string]string{}

	err := json.Unmarshal(payload, &binaries)
	Expect(err).NotTo(HaveOccurred())

	watcherPath = string(binaries["watcher"])
	locketBinPath = string(binaries["locket"])

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	logger = lagertest.NewTestLogger("test")
})

var _ = BeforeEach(func() {
	consulRunner.Start()
	consulRunner.WaitUntilReady()

	fakeCC = ghttp.NewServer()
	fakeBBS = ghttp.NewServer()

	watcherConfig = tpsconfig.DefaultWatcherConfig()
	watcherConfig.BBSAddress = fakeBBS.URL()
	watcherConfig.ConsulCluster = consulRunner.ConsulCluster()
	watcherConfig.CCBaseUrl = fmt.Sprintf(fakeCC.URL())
	watcherConfig.LagerConfig.LogLevel = "debug"
	watcherConfig.CCClientCert = "../../fixtures/watcher_cc_client.crt"
	watcherConfig.CCClientKey = "../../fixtures/watcher_cc_client.key"
	watcherConfig.CCCACert = "../../fixtures/watcher_cc_ca.crt"

	disableStartCheck = false
})

var _ = JustBeforeEach(func() {
	runner = tpsrunner.NewWatcher(string(watcherPath), watcherConfig)
	if disableStartCheck {
		runner.StartCheck = ""
	}
	watcher = ginkgomon.Invoke(runner)
	time.Sleep(1 * time.Second)
})

var _ = AfterEach(func() {
	fakeCC.Close()
	fakeBBS.Close()
	consulRunner.Stop()
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
