package envelope_extensions_test

import (
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EnvelopeExtensions", func() {
	var testAppUuid = &events.UUID{
		Low:  proto.Uint64(1),
		High: proto.Uint64(2),
	}

	Describe("GetAppId", func() {
		Context("HttpStartStop", func() {
			It("returns the App ID if it has one", func() {
				envelope := &events.Envelope{
					EventType:     events.Envelope_HttpStartStop.Enum(),
					HttpStartStop: &events.HttpStartStop{ApplicationId: testAppUuid},
				}
				appId := envelope_extensions.GetAppId(envelope)
				Expect(appId).To(Equal("01000000-0000-0000-0200-000000000000"))
			})
		})

		Context("LogMessage", func() {
			It("returns the App ID ", func() {
				envelope := &events.Envelope{
					EventType:  events.Envelope_LogMessage.Enum(),
					LogMessage: &events.LogMessage{AppId: proto.String("test-app-id")},
				}
				appId := envelope_extensions.GetAppId(envelope)
				Expect(appId).To(Equal("test-app-id"))
			})
		})

		Context("ContainerMetric", func() {
			It("returns the App ID ", func() {
				envelope := &events.Envelope{
					EventType:       events.Envelope_ContainerMetric.Enum(),
					ContainerMetric: &events.ContainerMetric{ApplicationId: proto.String("test-app-id")},
				}
				appId := envelope_extensions.GetAppId(envelope)
				Expect(appId).To(Equal("test-app-id"))
			})
		})
	})
})
