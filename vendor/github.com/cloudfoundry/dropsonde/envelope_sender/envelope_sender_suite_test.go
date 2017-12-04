package envelope_sender_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"testing"
)

func TestEnvelopeSender(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EnvelopeSender Suite")
}

func createTestEnvelope(envOrigin string) *events.Envelope {
	return &events.Envelope{
		Origin:     proto.String(envOrigin),
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
