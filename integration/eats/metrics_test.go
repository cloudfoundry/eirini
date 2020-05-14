package eats_test

import (
	"net"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"

	"code.cloudfoundry.org/eirini/integration/eats/fakes"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

//counterfeiter:generate -o fakes/fake_ingress_server.go ../../vendor/code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2/ingress.pb.go IngressServer

type BatchSenderStub func(server loggregator_v2.Ingress_BatchSenderServer) error

var _ = Describe("Metrics", func() {

	var (
		metricsConfigFile string
		metricsSession    *gexec.Session

		grpcServer *grpc.Server
		envelopes  chan *loggregator_v2.Envelope

		metricsCertPath, metricsKeyPath string
	)

	BeforeEach(func() {
		metricsCertPath, metricsKeyPath = util.GenerateKeyPair("metron")

		envelopes = make(chan *loggregator_v2.Envelope)
		var metronAddress string
		grpcServer, metronAddress = runMetronStub(metricsCertPath, metricsKeyPath, envelopes)

		config := &eirini.MetricsCollectorConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			LoggregatorAddress:               metronAddress,
			LoggregatorCertPath:              metricsCertPath,
			LoggregatorCAPath:                metricsCertPath,
			LoggregatorKeyPath:               metricsKeyPath,
			AppMetricsEmissionIntervalInSecs: 1,
		}
		metricsSession, metricsConfigFile = util.RunBinary(binPaths.MetricsCollector, config)
	})

	AfterEach(func() {
		if metricsSession != nil {
			metricsSession.Kill()
		}
		Expect(os.Remove(metricsConfigFile)).To(Succeed())

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
						Image: "eirini/dorini",
					},
				},
				MemoryMB: 200,
				DiskMB:   300,
			}
		})

		JustBeforeEach(func() {
			Expect(desireLRP(lrp).StatusCode).To(Equal(http.StatusAccepted))
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
				var (
					instanceIds []string
					envelope    *loggregator_v2.Envelope
				)
				for i := 0; i < lrp.NumInstances; i++ {
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
