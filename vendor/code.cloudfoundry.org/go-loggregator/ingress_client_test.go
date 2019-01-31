package loggregator_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-loggregator/runtimeemitter"
	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

// logCount is set to 3000 to ensure that the gRPC stream HAS to send messages
// instead of just buffering them. It seems to buffer up until 2000.
const logCount = 3000

//TestMain acts as the log emitter for gRPC SendRecv() test.
func TestMain(m *testing.M) {
	if os.Getenv("INGRESS_CLIENT_TEST_PROCESS") != "" {
		client, _ := buildIngressClient(os.Getenv("INGRESS_CLIENT_TEST_PROCESS"), time.Hour, false)
		for i := 0; i < logCount; i++ {
			client.EmitLog(fmt.Sprint("message", i))
		}
		client.CloseSend()
		return
	}

	os.Exit(m.Run())
}

var _ = Describe("IngressClient", func() {
	var (
		client *loggregator.IngressClient
		server *testIngressServer
		cancel func()
	)

	BeforeEach(func() {
		var err error
		server, err = newTestIngressServer(
			fixture("server.crt"),
			fixture("server.key"),
			fixture("CA.crt"),
		)
		Expect(err).NotTo(HaveOccurred())

		err = server.start()
		Expect(err).NotTo(HaveOccurred())

		client, cancel = buildIngressClient(server.addr, 50*time.Millisecond, false)
	})

	AfterEach(func() {
		cancel()
		server.stop()
	})

	It("sends in batches", func() {
		for i := 0; i < 10; i++ {
			client.EmitLog(
				"message",
				loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
				loggregator.WithStdout(),
			)
			time.Sleep(10 * time.Millisecond)
		}

		Eventually(func() int {
			var recv loggregator_v2.Ingress_BatchSenderServer
			Eventually(server.receivers, 10).Should(Receive(&recv))

			b, err := recv.Recv()
			if err != nil {
				return 0
			}

			return len(b.Batch)
		}).Should(BeNumerically(">", 1))
	})

	It("returns an error after context has been cancelled", func() {
		client, cancel := buildIngressClient(server.addr, time.Hour, false)
		cancel()
		go func(client *loggregator.IngressClient) {
			for range time.Tick(1 * time.Millisecond) {
				client.EmitLog(
					"message",
					loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
					loggregator.WithStdout(),
				)
			}
		}(client)

		Consistently(server.receivers).ShouldNot(Receive())
	})

	It("sends app logs", func() {
		client.EmitLog(
			"message",
			loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
			loggregator.WithStdout(),
		)
		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.SourceId).To(Equal("app-id"))
		Expect(env.InstanceId).To(Equal("source-instance"))

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		log := env.GetLog()
		Expect(log).NotTo(BeNil())
		Expect(log.Payload).To(Equal([]byte("message")))
		Expect(log.Type).To(Equal(loggregator_v2.Log_OUT))
	})

	It("sends logs", func() {
		client.EmitLog(
			"message",
			loggregator.WithSourceInfo("source-id", "source-type", "source-instance"),
			loggregator.WithStdout(),
		)
		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.SourceId).To(Equal("source-id"))
		Expect(env.InstanceId).To(Equal("source-instance"))

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		log := env.GetLog()
		Expect(log).NotTo(BeNil())
		Expect(log.Payload).To(Equal([]byte("message")))
		Expect(log.Type).To(Equal(loggregator_v2.Log_OUT))
	})

	It("sends app error logs", func() {
		client.EmitLog(
			"message",
			loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
		)

		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.SourceId).To(Equal("app-id"))
		Expect(env.InstanceId).To(Equal("source-instance"))

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		log := env.GetLog()
		Expect(log).NotTo(BeNil())
		Expect(log.Payload).To(Equal([]byte("message")))
		Expect(log.Type).To(Equal(loggregator_v2.Log_ERR))
	})

	It("sends app metrics", func() {
		client.EmitGauge(
			loggregator.WithGaugeValue("name-a", 1, "unit-a"),
			loggregator.WithGaugeValue("name-b", 2, "unit-b"),
			loggregator.WithEnvelopeTags(map[string]string{"some-tag": "some-tag-value"}),
			loggregator.WithGaugeAppInfo("app-id", 123),
		)

		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		metrics := env.GetGauge()
		Expect(metrics).NotTo(BeNil())
		Expect(env.SourceId).To(Equal("app-id"))
		Expect(env.InstanceId).To(Equal("123"))
		Expect(metrics.GetMetrics()).To(HaveLen(2))
		Expect(metrics.GetMetrics()["name-a"].Value).To(Equal(1.0))
		Expect(metrics.GetMetrics()["name-b"].Value).To(Equal(2.0))
		Expect(env.Tags["some-tag"]).To(Equal("some-tag-value"))
	})

	It("sends gauge metrics", func() {
		client.EmitGauge(
			loggregator.WithGaugeValue("name-a", 1, "unit-a"),
			loggregator.WithGaugeValue("name-b", 2, "unit-b"),
			loggregator.WithEnvelopeTags(map[string]string{"some-tag": "some-tag-value"}),
			loggregator.WithGaugeSourceInfo("source-id", "instance-id"),
		)

		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		metrics := env.GetGauge()
		Expect(metrics).NotTo(BeNil())
		Expect(env.SourceId).To(Equal("source-id"))
		Expect(env.InstanceId).To(Equal("instance-id"))
		Expect(metrics.GetMetrics()).To(HaveLen(2))
		Expect(metrics.GetMetrics()["name-a"].Value).To(Equal(1.0))
		Expect(metrics.GetMetrics()["name-b"].Value).To(Equal(2.0))
		Expect(env.Tags["some-tag"]).To(Equal("some-tag-value"))
	})

	It("sends timers", func() {
		stopTime := time.Now()
		startTime := stopTime.Add(-time.Minute)

		client.EmitTimer("http", startTime, stopTime,
			loggregator.WithEnvelopeTags(map[string]string{"some-tag": "some-tag-value"}),
			loggregator.WithTimerSourceInfo("source-id", "instance-id"),
		)

		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).ToNot(HaveOccurred())

		Expect(env.GetSourceId()).To(Equal("source-id"))
		Expect(env.GetInstanceId()).To(Equal("instance-id"))
		Expect(env.Tags["some-tag"]).To(Equal("some-tag-value"))

		timer := env.GetTimer()
		Expect(timer).ToNot(BeNil())
		Expect(timer.GetName()).To(Equal("http"))
		Expect(timer.GetStart()).To(Equal(startTime.UnixNano()))
		Expect(timer.GetStop()).To(Equal(stopTime.UnixNano()))
	})

	It("sends envelopes", func() {
		stopTime := time.Now()
		startTime := stopTime.Add(-time.Minute)

		client.Emit(&loggregator_v2.Envelope{
			Timestamp:  stopTime.UnixNano(),
			SourceId:   "source-id",
			InstanceId: "instance-id",
			Tags: map[string]string{
				"some-tag": "some-tag-value",
			},
			Message: &loggregator_v2.Envelope_Timer{
				Timer: &loggregator_v2.Timer{
					Name:  "http",
					Start: startTime.UnixNano(),
					Stop:  stopTime.UnixNano(),
				},
			},
		})
		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).ToNot(HaveOccurred())

		Expect(env.GetSourceId()).To(Equal("source-id"))
		Expect(env.GetInstanceId()).To(Equal("instance-id"))
		Expect(env.Tags["some-tag"]).To(Equal("some-tag-value"))

		timer := env.GetTimer()
		Expect(timer).ToNot(BeNil())
		Expect(timer.GetName()).To(Equal("http"))
		Expect(timer.GetStart()).To(Equal(startTime.UnixNano()))
		Expect(timer.GetStop()).To(Equal(stopTime.UnixNano()))
	})

	It("works with the runtime emitter", func() {
		// This test is to ensure that the v2 client satisfies the
		// runtimeemitter.Sender interface. If it does not satisfy the
		// runtimeemitter.Sender interface this test will force a compile time
		// error.
		runtimeemitter.New(client)
	})

	DescribeTable("emitting different envelope types", func(emit func()) {
		emit()

		env, err := getEnvelopeAt(server.receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.Tags["string"]).To(Equal("client-string-tag"), "The client tag for string was not set properly")
		Expect(env.Tags["envelope-string"]).To(Equal("envelope-string-tag"), "The envelope tag for string was not set properly")
	},
		Entry("logs", func() {
			client.EmitLog(
				"message",
				loggregator.WithEnvelopeTag("envelope-string", "envelope-string-tag"),
			)
		}),
		Entry("gauge", func() {
			client.EmitGauge(
				loggregator.WithGaugeValue("gauge-name", 123.4, "some-unit"),
				loggregator.WithEnvelopeTag("envelope-string", "envelope-string-tag"),
			)
		}),
		Entry("counter", func() {
			client.EmitCounter(
				"foo",
				loggregator.WithEnvelopeTag("envelope-string", "envelope-string-tag"),
			)
		}),
	)

	It("sets the counter's delta to the given value", func() {
		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{},
			},
		}
		loggregator.WithDelta(99)(e)
		Expect(e.GetCounter().GetDelta()).To(Equal(uint64(99)))
	})

	It("sets the app info for a counter", func() {
		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{},
			},
		}
		loggregator.WithCounterAppInfo("some-guid", 101)(e)
		Expect(e.GetSourceId()).To(Equal("some-guid"))
		Expect(e.GetInstanceId()).To(Equal("101"))
	})

	It("sets the source info for a counter", func() {
		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{},
			},
		}
		loggregator.WithCounterSourceInfo("source-id", "instance-id")(e)
		Expect(e.GetSourceId()).To(Equal("source-id"))
		Expect(e.GetInstanceId()).To(Equal("instance-id"))
	})

	It("sets the title and body of an event envelope", func() {
		Eventually(func() error {
			return client.EmitEvent(
				context.Background(),
				"some-title",
				"some-body",
			)
		}).Should(Succeed())

		var envelopeBatch *loggregator_v2.EnvelopeBatch
		Eventually(server.sendReceiver).Should(Receive(&envelopeBatch))

		env := envelopeBatch.GetBatch()[0]
		Expect(env.GetEvent()).ToNot(BeNil())
		Expect(env.GetEvent().GetTitle()).To(Equal("some-title"))
		Expect(env.GetEvent().GetBody()).To(Equal("some-body"))
	})

	It("sets the source info for an event", func() {
		Eventually(func() error {
			return client.EmitEvent(
				context.Background(),
				"some-title",
				"some-body",
				loggregator.WithEventSourceInfo("source-id", "instance-id"),
			)
		}).Should(Succeed())
		var envelopeBatch *loggregator_v2.EnvelopeBatch
		Eventually(server.sendReceiver).Should(Receive(&envelopeBatch))

		env := envelopeBatch.GetBatch()[0]
		Expect(env.GetEvent()).ToNot(BeNil())
		Expect(env.GetSourceId()).To(Equal("source-id"))
		Expect(env.GetInstanceId()).To(Equal("instance-id"))
	})

	// So this test is a bit... crazy. We want to ensure that gRPC gets to
	// flush its buffer. However, gRPC is pretty good about getting data out
	// of its process, therefore we have to fight it. We need to run the
	// sending side on a different process and have that process exit to
	// ensure we are actually excercising the need for CloseAndRecv().
	It("flushes current batch and sends", func() {
		var wg sync.WaitGroup
		wg.Add(1)
		defer wg.Wait()
		go func() {
			defer wg.Done()
			defer GinkgoRecover()
			es, err := getEnvelopesN(server.receivers, logCount)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(es)).To(Equal(logCount))
			server.stop()
		}()

		path, err := os.Executable()
		Expect(err).ToNot(HaveOccurred())
		cmd := exec.Command(path)
		cmd.Env = []string{
			"INGRESS_CLIENT_TEST_PROCESS=" + server.addr,
		}
		Expect(cmd.Start()).To(Succeed())
		cmd.Wait()
	}, 5)

	It("does not block on an empty buffer", func(done Done) {
		defer close(done)

		err := client.CloseSend()
		Expect(err).ToNot(HaveOccurred())
	})
})

func getEnvelopesN(receivers chan loggregator_v2.Ingress_BatchSenderServer, n int) ([]*loggregator_v2.Envelope, error) {
	var recv loggregator_v2.Ingress_BatchSenderServer
	Eventually(receivers, 10).Should(Receive(&recv))

	var results []*loggregator_v2.Envelope
	for {
		envBatch, err := recv.Recv()
		if err != nil {
			// Return what you have. The connection may be killed
			// (delibarately) by the other process.
			return results, nil
		}

		results = append(results, envBatch.Batch...)
		if len(results) >= n {
			return results, nil
		}
	}
}

func getEnvelopeAt(receivers chan loggregator_v2.Ingress_BatchSenderServer, idx int) (*loggregator_v2.Envelope, error) {
	var recv loggregator_v2.Ingress_BatchSenderServer
	Eventually(receivers, 10).Should(Receive(&recv))

	envBatch, err := recv.Recv()
	if err != nil {
		return nil, err
	}

	if len(envBatch.Batch) < 1 {
		return nil, errors.New("no envelopes")
	}

	return envBatch.Batch[idx], nil
}

func buildIngressClient(serverAddr string, flushInterval time.Duration, addContext bool) (*loggregator.IngressClient, func()) {
	tlsConfig, err := loggregator.NewIngressTLSConfig(
		fixture("CA.crt"),
		fixture("client.crt"),
		fixture("client.key"),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	opts := []loggregator.IngressOption{
		loggregator.WithAddr(serverAddr),
		loggregator.WithBatchFlushInterval(flushInterval),
		loggregator.WithTag("string", "client-string-tag"),
	}

	if addContext {
		opts = append(opts, loggregator.WithContext(ctx))
	}

	client, err := loggregator.NewIngressClient(
		tlsConfig,
		opts...,
	)
	if err != nil {
		panic(err)
	}

	return client, cancel
}
