package emitter_test

import (
	"github.com/cloudfoundry/dropsonde/emitter"

	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	uuid "github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type unknownEvent struct{}

func (*unknownEvent) ProtoMessage() {}

var _ = Describe("EventFormatter", func() {
	Describe("wrap", func() {
		var origin string

		BeforeEach(func() {
			origin = "testEventFormatter/42"
		})

		It("works with ValueMetric events", func() {
			testEvent := &events.ValueMetric{Name: proto.String("test-name")}

			envelope, _ := emitter.Wrap(testEvent, origin)
			Expect(envelope.GetEventType()).To(Equal(events.Envelope_ValueMetric))
			Expect(envelope.GetValueMetric()).To(Equal(testEvent))
		})

		It("works with CounterEvent events", func() {
			testEvent := &events.CounterEvent{Name: proto.String("test-counter")}

			envelope, _ := emitter.Wrap(testEvent, origin)
			Expect(envelope.GetEventType()).To(Equal(events.Envelope_CounterEvent))
			Expect(envelope.GetCounterEvent()).To(Equal(testEvent))
		})

		It("works with HttpStartStop events", func() {
			testEvent := &events.HttpStartStop{
				StartTimestamp: proto.Int64(200),
				StopTimestamp:  proto.Int64(500),
				RequestId: &events.UUID{
					Low:  proto.Uint64(200),
					High: proto.Uint64(300),
				},
				PeerType:      events.PeerType_Client.Enum(),
				Method:        events.Method_GET.Enum(),
				Uri:           proto.String("http://some.example.com"),
				RemoteAddress: proto.String("http://remote.address"),
				UserAgent:     proto.String("some user agent"),
				ContentLength: proto.Int64(200),
				StatusCode:    proto.Int32(200),
			}

			envelope, err := emitter.Wrap(testEvent, origin)
			Expect(err).ToNot(HaveOccurred())
			Expect(envelope.GetEventType()).To(Equal(events.Envelope_HttpStartStop))
			Expect(envelope.GetHttpStartStop()).To(Equal(testEvent))
		})

		It("errors with unknown events", func() {
			envelope, err := emitter.Wrap(new(unknownEvent), origin)
			Expect(envelope).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("checks that origin is non-empty", func() {
			id, _ := uuid.NewV4()
			malformedOrigin := ""
			testEvent := &events.HttpStartStop{RequestId: factories.NewUUID(id)}
			envelope, err := emitter.Wrap(testEvent, malformedOrigin)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Event not emitted due to missing origin information"))
			Expect(envelope).To(BeNil())
		})

		Context("with a known event type", func() {
			var testEvent events.Event

			BeforeEach(func() {
				id, _ := uuid.NewV4()
				testEvent = &events.HttpStartStop{RequestId: factories.NewUUID(id)}
			})

			It("contains the origin", func() {
				envelope, _ := emitter.Wrap(testEvent, origin)
				Expect(envelope.GetOrigin()).To(Equal("testEventFormatter/42"))
			})

			Context("when the origin is empty", func() {
				It("errors with a helpful message", func() {
					envelope, err := emitter.Wrap(testEvent, "")
					Expect(envelope).To(BeNil())
					Expect(err.Error()).To(Equal("Event not emitted due to missing origin information"))
				})
			})

			It("sets the timestamp to now", func() {
				envelope, _ := emitter.Wrap(testEvent, origin)
				Expect(time.Unix(0, envelope.GetTimestamp())).To(BeTemporally("~", time.Now(), 100*time.Millisecond))
			})
		})
	})
})
