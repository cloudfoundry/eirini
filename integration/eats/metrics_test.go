package eats_test

import (
	"net"
	"net/http"
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini"
	"gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini/integration/eats/fakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

//go:generate counterfeiter -o fakes/fake_ingress_server.go ../../vendor/code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2/ingress.pb.go IngressServer

type BatchSenderStub func(server loggregator_v2.Ingress_BatchSenderServer) error

var _ = Describe("Metrics", func() {

	var (
		metricsConfigFile string
		opiConfigFile     string
		metricsSession    *gexec.Session
		opiSession        *gexec.Session

		httpClient *http.Client
		opiURL     string

		grpcServer *grpc.Server
		envelopes  chan *loggregator_v2.Envelope

		metricsCertPath, metricsKeyPath     string
		localhostCertPath, localhostKeyPath string
	)

	BeforeEach(func() {
		metricsCertPath, metricsKeyPath = generateKeyPair("metron")
		localhostCertPath, localhostKeyPath = generateKeyPair("localhost")

		var err error
		httpClient, err = makeTestHTTPClient(localhostCertPath, localhostKeyPath)
		Expect(err).ToNot(HaveOccurred())

		envelopes = make(chan *loggregator_v2.Envelope)
		var metronAddress string
		grpcServer, metronAddress = runMetronStub(metricsCertPath, metricsKeyPath, envelopes)

		metricsSession, metricsConfigFile = runMetricsCollector(metricsCertPath, metricsKeyPath, metronAddress)

		opiSession, opiConfigFile, opiURL = runOpi(localhostCertPath, localhostKeyPath)
		waitOpiReady(httpClient, opiURL)
	})

	AfterEach(func() {
		if metricsSession != nil {
			metricsSession.Kill()
		}
		if opiSession != nil {
			opiSession.Kill()
		}
		Expect(os.Remove(metricsConfigFile)).To(Succeed())
		Expect(os.Remove(opiConfigFile)).To(Succeed())

		grpcServer.Stop()
	})

	Context("When an app is running", func() {
		var lrp cf.DesireLRPRequest

		BeforeEach(func() {
			lrp = cf.DesireLRPRequest{
				GUID:         "the-app-guid",
				Version:      "0.0.0",
				NumInstances: 1,
				Ports:        []int32{8080},
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image: "eirini/notdora",
					},
				},
				MemoryMB: 200,
				DiskMB:   300,
			}
		})

		JustBeforeEach(func() {
			resp, err := desireLRP(httpClient, opiURL, lrp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("reports its metrics", func() {
			var envelope *loggregator_v2.Envelope
			Eventually(envelopes).Should(Receive(&envelope))
			Expect(envelope.SourceId).To(Equal("the-app-guid"))
			Expect(envelope.InstanceId).To(Equal("0"))

			checkMetrics(envelope, lrp.MemoryMB, lrp.DiskMB)
		})

		Context("and has more than one instance", func() {
			BeforeEach(func() {
				lrp.NumInstances = 3
			})

			It("reports metrics for each instance", func() {
				var instanceIds []string
				for i := 0; i < lrp.NumInstances; i++ {
					var envelope *loggregator_v2.Envelope
					Eventually(envelopes).Should(Receive(&envelope))

					Expect(envelope.SourceId).To(Equal("the-app-guid"))
					checkMetrics(envelope, lrp.MemoryMB, lrp.DiskMB)

					instanceIds = append(instanceIds, envelope.InstanceId)
				}
				Expect(instanceIds).To(ConsistOf("0", "1", "2"))
			})
		})
	})
})

func newGrpcServer(cert, key, ca, addr string) (*grpc.Server, net.Listener) {
	creds, err := plumbing.NewServerCredentials(cert, key, ca)
	Expect(err).ToNot(HaveOccurred())

	listener, err := net.Listen("tcp", addr)
	Expect(err).ToNot(HaveOccurred())
	return grpc.NewServer(grpc.Creds(creds)), listener
}

func runMetronStub(certPath, keyPath string, envelopes chan *loggregator_v2.Envelope) (*grpc.Server, string) {
	grpcServer, grpcListener := newGrpcServer(certPath, keyPath, certPath, "localhost:0")
	ingressServer := new(fakes.FakeIngressServer)
	ingressServer.BatchSenderStub = batchSenderStub(envelopes)
	loggregator_v2.RegisterIngressServer(grpcServer, ingressServer)
	go grpcServer.Serve(grpcListener) //nolint:errcheck

	return grpcServer, grpcListener.Addr().String()
}

func batchSenderStub(envelopes chan *loggregator_v2.Envelope) BatchSenderStub {
	return func(server loggregator_v2.Ingress_BatchSenderServer) error {
		defer close(envelopes)
		for {
			batch, err := server.Recv()
			if err != nil {
				return nil
			}
			for _, envelope := range batch.Batch {
				envelopes <- envelope
			}
		}
	}
}

func runMetricsCollector(certPath, keyPath, metronAddress string) (*gexec.Session, string) {
	binaryPath, err := gexec.Build("code.cloudfoundry.org/eirini/cmd/metrics-collector")
	Expect(err).NotTo(HaveOccurred())

	config := &eirini.MetricsCollectorConfig{
		KubeConfig: eirini.KubeConfig{
			Namespace:  fixture.Namespace,
			ConfigPath: fixture.KubeConfigPath,
		},
		LoggregatorAddress:               metronAddress,
		LoggregatorCertPath:              certPath,
		LoggregatorCAPath:                certPath,
		LoggregatorKeyPath:               keyPath,
		AppMetricsEmissionIntervalInSecs: 1,
	}

	configBytes, err := yaml.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	configFile := writeTempFile(configBytes, "config.yaml")
	command := exec.Command(binaryPath, "-c", configFile) // #nosec G204
	metricsSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return metricsSession, configFile
}

func checkMetrics(envelope *loggregator_v2.Envelope, memoryQuota, diskQuota int64) {
	metrics := envelope.GetGauge().GetMetrics()
	Expect(metrics["cpu"].Unit).To(Equal("percentage"))
	Expect(metrics["memory"].Unit).To(Equal("bytes"))
	Expect(metrics["memory_quota"].Value).To(Equal(mbToByte(memoryQuota)))
	Expect(metrics["memory_quota"].Unit).To(Equal("bytes"))
	Expect(metrics["disk"].Unit).To(Equal("bytes"))
	Expect(metrics["disk_quota"].Value).To(Equal(mbToByte(diskQuota)))
	Expect(metrics["disk_quota"].Unit).To(Equal("bytes"))
}

func mbToByte(mb int64) float64 {
	return float64(mb * 1000 * 1000)
}
