package envelope_sender_test

import (
	"errors"

	"github.com/cloudfoundry/dropsonde/emitter/fake"

	"github.com/cloudfoundry/dropsonde/envelope_sender"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EnvelopeSender", func() {
	var (
		emitter   *fake.FakeEventEmitter
		sender    *envelope_sender.EnvelopeSender
		envOrigin string
	)

	BeforeEach(func() {
		envOrigin = "original-origin"
		emitter = fake.NewFakeEventEmitter("origin")
		sender = envelope_sender.NewEnvelopeSender(emitter)
	})

	It("sends an Envelope to its emitter", func() {
		err := sender.SendEnvelope(createTestEnvelope(envOrigin))
		Expect(err).NotTo(HaveOccurred())

		Expect(emitter.GetEnvelopes()).To(HaveLen(1))
		envelope := emitter.GetEnvelopes()[0]
		metric := envelope.ValueMetric
		Expect(metric.GetName()).To(Equal("metric-name"))
		Expect(metric.GetValue()).To(BeNumerically("==", 42))
		Expect(metric.GetUnit()).To(Equal("answers"))
		Expect(envelope.Origin).To(Equal(proto.String(envOrigin)))
	})

	It("returns an error if it can't send metric value", func() {
		emitter.ReturnError = errors.New("some error")

		err := sender.SendEnvelope(createTestEnvelope(envOrigin))
		Expect(emitter.GetMessages()).To(HaveLen(0))
		Expect(err.Error()).To(Equal("some error"))
	})

})
