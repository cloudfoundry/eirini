package format_test

import (
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/format/formatfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Envelope", func() {
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
	})

	Describe("Marshal", func() {
		It("can successfully marshal a model object envelope", func() {
			task := model_helpers.NewValidTask("some-guid")
			encoded, err := format.MarshalEnvelope(format.PROTO, task)
			Expect(err).NotTo(HaveOccurred())

			Expect(format.EnvelopeFormat(encoded[0])).To(Equal(format.PROTO))

			var newTask models.Task
			modelErr := proto.Unmarshal(encoded[2:], &newTask)
			Expect(modelErr).To(BeNil())

			Expect(*task).To(Equal(newTask))
		})

		It("returns an error when marshalling when the envelope doesn't support the model", func() {
			model := &formatfakes.FakeVersioner{}
			_, err := format.MarshalEnvelope(format.PROTO, model)
			Expect(err).To(MatchError("Model object incompatible with envelope format"))
		})
	})

	Describe("Unmarshal", func() {
		It("can marshal and unmarshal a task without losing data", func() {
			task := model_helpers.NewValidTask("some-guid")
			payload, err := format.MarshalEnvelope(format.PROTO, task)
			Expect(err).NotTo(HaveOccurred())

			resultingTask := new(models.Task)
			err = format.UnmarshalEnvelope(logger, payload, resultingTask)
			Expect(err).NotTo(HaveOccurred())

			Expect(*resultingTask).To(BeEquivalentTo(*task))
		})

		It("returns an error when the serialization format is unknown", func() {
			model := &formatfakes.FakeVersioner{}
			payload := []byte{byte(format.EnvelopeFormat(99)), byte(format.V0), '{', '}'}
			err := format.UnmarshalEnvelope(logger, payload, model)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the json payload is invalid", func() {
			model := &formatfakes.FakeVersioner{}
			payload := []byte{byte(format.JSON), byte(format.V0), 'f', 'o', 'o'}
			err := format.UnmarshalEnvelope(logger, payload, model)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the protobuf payload is invalid", func() {
			model := model_helpers.NewValidTask("foo")
			payload := []byte{byte(format.PROTO), byte(format.V0), 'f', 'o', 'o'}
			err := format.UnmarshalEnvelope(logger, payload, model)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when unmarshalling when the model doesn't match the envelope", func() {
			task := model_helpers.NewValidTask("some-guid")
			payload, err := format.MarshalEnvelope(format.PROTO, task)
			Expect(err).NotTo(HaveOccurred())

			model := &formatfakes.FakeVersioner{}
			err = format.UnmarshalEnvelope(logger, payload, model)
			Expect(err).To(MatchError("Model object incompatible with envelope format"))
		})
	})
})

func bytesForEnvelope(f format.EnvelopeFormat, v format.Version, payloads ...string) []byte {
	env := []byte{byte(f), byte(v)}
	for i := range payloads {
		env = append(env, []byte(payloads[i])...)
	}
	return env
}
