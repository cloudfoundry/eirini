package main_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var (
	listenerPath string

	bbsURL  *url.URL
	fakeBBS *ghttp.Server
)

var natsPort int

var consulRunner *consulrunner.ClusterRunner
var consulClient consuladapter.Client

func TestListener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Listener Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	listener, err := gexec.Build("code.cloudfoundry.org/nsync/cmd/nsync-listener", "-race")
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"listener": listener,
	})
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	binaries := map[string]string{}

	err := json.Unmarshal(payload, &binaries)
	Expect(err).NotTo(HaveOccurred())

	listenerPath = string(binaries["listener"])

	natsPort = 4001 + GinkgoParallelNode()

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)
})

var _ = BeforeEach(func() {
	consulRunner.Start()
	consulRunner.WaitUntilReady()
	consulClient = consulRunner.NewClient()

	fakeBBS = ghttp.NewServer()
	fakeBBS.AllowUnhandledRequests = false
})

var _ = AfterEach(func() {
	consulRunner.Stop()
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
