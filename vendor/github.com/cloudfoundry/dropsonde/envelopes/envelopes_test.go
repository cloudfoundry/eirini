package envelopes_test

import (
	"time"

	"github.com/cloudfoundry/dropsonde/envelopes"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/envelope_sender/fake"
)

var _ = Describe("Metrics", func() {
	var fakeEnvelopeSender *fake.FakeEnvelopeSender

	BeforeEach(func() {
		fakeEnvelopeSender = fake.NewFakeEnvelopeSender()
		envelopes.Initialize(fakeEnvelopeSender)
	})

	It("delegates SendEnvelope", func() {
		testEnvelope := createTestEnvelope()

		envelopes.SendEnvelope(testEnvelope)

		Expect(fakeEnvelopeSender.GetEnvelopes()).To(HaveLen(1))
		Expect(fakeEnvelopeSender.GetEnvelopes()[0]).To(BeEquivalentTo(testEnvelope))
	})

	Context("when Envelope Sender is not initialized", func() {

		BeforeEach(func() {
			envelopes.Initialize(nil)
		})

		It("SendEnvelope is a no-op", func() {
			err := envelopes.SendEnvelope(createTestEnvelope())

			Expect(err).ToNot(HaveOccurred())
		})
	})

})

func createTestEnvelope() *events.Envelope {
	return &events.Envelope{
		Origin:     proto.String("some-origin"),
		EventType:  events.Envelope_ValueMetric.Enum(),
		Timestamp:  proto.Int64(time.Now().Unix() * 1000),
		Deployment: proto.String("some-deployment"),
		Job:        proto.String("some-job"),
		Index:      proto.String("some-index"),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("answers"),
		},
	}
}
