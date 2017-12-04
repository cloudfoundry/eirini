package integration_test

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	lock           sync.RWMutex
	receivedEvents []eventTracker
	udpListener    net.PacketConn
)

// these tests need to be invoked individually from an external script,
// since environment variables need to be set/unset before starting the tests
var _ = Describe("Autowire End-to-End", func() {
	Context("with standard initialization", func() {
		origin := []string{"test-origin"}

		BeforeEach(func() {
			var err error
			udpListener, err = net.ListenPacket("udp4", ":3457")
			Expect(err).ToNot(HaveOccurred())

			go listenForEvents(origin)
			dropsonde.Initialize("localhost:3457", origin...)
			sender := metric_sender.NewMetricSender(dropsonde.AutowiredEmitter())
			batcher := metricbatcher.New(sender, 100*time.Millisecond)
			metrics.Initialize(sender, batcher)
		})

		AfterEach(func() {
			udpListener.Close()
		})

		It("emits HTTP client/server events", func() {
			httpListener, err := net.Listen("tcp", "localhost:0")
			Expect(err).ToNot(HaveOccurred())
			defer httpListener.Close()
			httpHandler := dropsonde.InstrumentedHandler(FakeHandler{})
			go http.Serve(httpListener, httpHandler)

			_, err = http.Get("http://" + httpListener.Addr().String())
			Expect(err).ToNot(HaveOccurred())

			metrics.SendValue("TestMetric", 0, "")
			metrics.IncrementCounter("TestIncrementCounter")
			metrics.BatchIncrementCounter("TestBatchedCounter")

			expectedEvents := []eventTracker{
				{eventType: "HttpStartStop", name: "Client"},
				{eventType: "HttpStartStop", name: "Server"},
				{eventType: "ValueMetric", name: "numCPUS"},
				{eventType: "ValueMetric", name: "TestMetric"},
				{eventType: "CounterEvent", name: "TestIncrementCounter"},
				{eventType: "CounterEvent", name: "TestBatchedCounter"},
			}

			for _, tracker := range expectedEvents {
				Eventually(func() []eventTracker { lock.Lock(); defer lock.Unlock(); return receivedEvents }, 5).Should(ContainElement(tracker))
			}
		})
	})
})

func listenForEvents(origin []string) {
	for {
		buffer := make([]byte, 1024)
		n, _, err := udpListener.ReadFrom(buffer)
		if err != nil {
			return
		}

		if n == 0 {
			panic("Received empty packet")
		}
		envelope := new(events.Envelope)
		err = proto.Unmarshal(buffer[0:n], envelope)
		if err != nil {
			panic(err)
		}

		var eventId = envelope.GetEventType().String()

		tracker := eventTracker{eventType: eventId}

		switch envelope.GetEventType() {
		case events.Envelope_HttpStartStop:
			tracker.name = envelope.GetHttpStartStop().GetPeerType().String()
		case events.Envelope_ValueMetric:
			tracker.name = envelope.GetValueMetric().GetName()
		case events.Envelope_CounterEvent:
			tracker.name = envelope.GetCounterEvent().GetName()
		default:
			panic("Unexpected message type: " + envelope.GetEventType().String())

		}

		if envelope.GetOrigin() != strings.Join(origin, "/") {
			panic("origin not as expected")
		}

		func() {
			lock.Lock()
			defer lock.Unlock()
			receivedEvents = append(receivedEvents, tracker)
		}()
	}
}

type eventTracker struct {
	eventType string
	name      string
}

type FakeHandler struct{}

func (fh FakeHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(rw, "Hello")
}

type FakeRoundTripper struct{}

func (frt FakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, nil
}
