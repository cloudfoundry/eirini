package main_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/locket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
	"time"
)

var (
	logger        lager.Logger
	portAllocator portauthority.PortAllocator

	client            bbs.InternalClient
	bbsBinPath        string
	bbsAddress        string
	bbsHealthAddress  string
	bbsPort           uint16
	bbsURL            *url.URL
	bbsConfig         bbsconfig.BBSConfig
	bbsRunner         *ginkgomon.Runner
	bbsProcess        ifrit.Process
	consulRunner      *consulrunner.ClusterRunner
	consulClient      consuladapter.Client
	consulHelper      *test_helpers.ConsulHelper
	auctioneerServer  *ghttp.Server
	testMetricsChan   chan *loggregator_v2.Envelope
	locketBinPath     string
	testIngressServer *testhelpers.TestIngressServer

	signalMetricsChan chan struct{}
	sqlProcess        ifrit.Process
	sqlRunner         sqlrunner.SQLRunner
)

func TestBBS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BBS Cmd Suite")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		bbsPath, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
		Expect(err).NotTo(HaveOccurred())

		locketPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
		Expect(err).NotTo(HaveOccurred())

		return []byte(strings.Join([]string{bbsPath, locketPath}, ","))
	},
	func(binPaths []byte) {
		grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
		startPort := 1050 * GinkgoParallelNode()
		portRange := 1000
		var err error
		portAllocator, err = portauthority.New(startPort, startPort+portRange)
		Expect(err).NotTo(HaveOccurred())

		path := string(binPaths)
		bbsBinPath = strings.Split(path, ",")[0]
		locketBinPath = strings.Split(path, ",")[1]

		SetDefaultEventuallyTimeout(15 * time.Second)

		dbName := fmt.Sprintf("diego_%d", GinkgoParallelNode())
		sqlRunner = test_helpers.NewSQLRunner(dbName)
		sqlProcess = ginkgomon.Invoke(sqlRunner)

		consulStartingPort, err := portAllocator.ClaimPorts(consulrunner.PortOffsetLength)
		Expect(err).NotTo(HaveOccurred())

		consulRunner = consulrunner.NewClusterRunner(
			consulrunner.ClusterRunnerConfig{
				StartingPort: int(consulStartingPort),
				NumNodes:     1,
				Scheme:       "http",
			},
		)

		consulRunner.Start()
		consulRunner.WaitUntilReady()
	},
)

var _ = SynchronizedAfterSuite(func() {
	ginkgomon.Kill(sqlProcess)

	if consulRunner != nil {
		consulRunner.Stop()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	var err error
	logger = lagertest.NewTestLogger("test")
	fixturesPath := path.Join(os.Getenv("GOPATH"), "src/code.cloudfoundry.org/bbs/cmd/bbs/fixtures")

	consulRunner.Reset()
	consulClient = consulRunner.NewClient()

	metronCAFile := path.Join(fixturesPath, "metron", "CA.crt")
	metronClientCertFile := path.Join(fixturesPath, "metron", "client.crt")
	metronClientKeyFile := path.Join(fixturesPath, "metron", "client.key")
	metronServerCertFile := path.Join(fixturesPath, "metron", "metron.crt")
	metronServerKeyFile := path.Join(fixturesPath, "metron", "metron.key")

	auctioneerServer = ghttp.NewServer()
	auctioneerServer.UnhandledRequestStatusCode = http.StatusAccepted
	auctioneerServer.AllowUnhandledRequests = true

	bbsPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	bbsAddress = fmt.Sprintf("127.0.0.1:%d", bbsPort)

	bbsHealthPort, err := portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	bbsHealthAddress = fmt.Sprintf("127.0.0.1:%d", bbsHealthPort)

	bbsURL = &url.URL{
		Scheme: "https",
		Host:   bbsAddress,
	}

	testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
	Expect(err).NotTo(HaveOccurred())

	testIngressServer.Start()

	metricsPort, err := strconv.Atoi(strings.TrimPrefix(testIngressServer.Addr(), "127.0.0.1:"))
	Expect(err).NotTo(HaveOccurred())

	receiversChan := testIngressServer.Receivers()

	testMetricsChan, signalMetricsChan = testhelpers.TestMetricChan(receiversChan)

	serverCaFile := path.Join(fixturesPath, "green-certs", "server-ca.crt")
	clientCertFile := path.Join(fixturesPath, "green-certs", "client.crt")
	clientKeyFile := path.Join(fixturesPath, "green-certs", "client.key")
	client, err = bbs.NewClient(bbsURL.String(), serverCaFile, clientCertFile, clientKeyFile, 0, 0)
	Expect(err).ToNot(HaveOccurred())

	bbsConfig = bbsconfig.BBSConfig{
		SessionName:                     "bbs",
		CommunicationTimeout:            durationjson.Duration(10 * time.Second),
		RequireSSL:                      true,
		DesiredLRPCreationTimeout:       durationjson.Duration(1 * time.Minute),
		ExpireCompletedTaskDuration:     durationjson.Duration(2 * time.Minute),
		ExpirePendingTaskDuration:       durationjson.Duration(30 * time.Minute),
		EnableConsulServiceRegistration: false,
		KickTaskDuration:                durationjson.Duration(30 * time.Second),
		LockTTL:                         durationjson.Duration(locket.DefaultSessionTTL),
		LockRetryInterval:               durationjson.Duration(locket.RetryInterval),
		ConvergenceWorkers:              20,
		UpdateWorkers:                   1000,
		TaskCallbackWorkers:             1000,
		MaxOpenDatabaseConnections:      200,
		MaxIdleDatabaseConnections:      200,
		AuctioneerRequireTLS:            false,
		RepClientSessionCacheSize:       0,
		RepRequireTLS:                   false,

		ListenAddress:     bbsAddress,
		AdvertiseURL:      bbsURL.String(),
		AuctioneerAddress: auctioneerServer.URL(),
		ConsulCluster:     consulRunner.ConsulCluster(),

		DatabaseDriver:                 sqlRunner.DriverName(),
		DatabaseConnectionString:       sqlRunner.ConnectionString(),
		DetectConsulCellRegistrations:  true,
		ReportInterval:                 durationjson.Duration(time.Second / 2),
		HealthAddress:                  bbsHealthAddress,
		CellRegistrationsLocketEnabled: false,
		LocksLocketEnabled:             false,

		EncryptionConfig: encryption.EncryptionConfig{
			EncryptionKeys: map[string]string{"label": "key"},
			ActiveKeyLabel: "label",
		},
		ConvergeRepeatInterval: durationjson.Duration(time.Hour),
		UUID:                   "bbs-bosh-boshy-bosh-bosh",

		CaFile:   serverCaFile,
		CertFile: path.Join(fixturesPath, "green-certs", "server.crt"),
		KeyFile:  path.Join(fixturesPath, "green-certs", "server.key"),

		LagerConfig: lagerflags.LagerConfig{
			LogLevel: lagerflags.DEBUG,
		},

		LoggregatorConfig: diego_logging_client.Config{
			BatchFlushInterval: 10 * time.Millisecond,
			BatchMaxSize:       1,
			UseV2API:           true,
			APIPort:            metricsPort,
			CACertPath:         metronCAFile,
			KeyPath:            metronClientKeyFile,
			CertPath:           metronClientCertFile,
		},
	}
	consulHelper = test_helpers.NewConsulHelper(logger, consulClient)
})

var _ = AfterEach(func() {
	ginkgomon.Kill(bbsProcess)

	// Make sure the healthcheck server is really gone before trying to start up
	// the bbs again in another test.
	Eventually(func() error {
		conn, err := net.Dial("tcp", bbsHealthAddress)
		if err == nil {
			conn.Close()
			return nil
		}

		return err
	}).Should(HaveOccurred())

	auctioneerServer.Close()
	testIngressServer.Stop()
	close(signalMetricsChan)

	sqlRunner.Reset()
})
