package v1_test

import (
	"time"

	"code.cloudfoundry.org/go-loggregator"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/v1"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeClient", func() {
	var (
		client *v1.Client
	)

	Describe("v1 and v2 interface compatibility", func() {
		Context("v1 client", func() {
			var (
				originalEventEmitter dropsonde.EventEmitter
				spyEmitter           *SpyEventEmitter
			)

			BeforeEach(func() {
				dropsonde.Initialize("dst", "origin")
				originalEventEmitter = dropsonde.DefaultEmitter
				spyEmitter = NewSpyEventEmitter("my-origin")
				dropsonde.DefaultEmitter = spyEmitter

				client, _ = v1.NewClient()
			})

			AfterEach(func() {
				dropsonde.DefaultEmitter = originalEventEmitter
			})

			Describe("EmitLog", func() {
				It("emits a log with a message", func() {
					client.EmitLog("my message")

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetEventType()).To(Equal(events.Envelope_LogMessage))
					Expect(env.GetOrigin()).To(Equal("my-origin"))
					Expect(env.GetTimestamp()).To(BeNumerically("~", time.Now().UnixNano(), time.Second))

					message := env.GetLogMessage()
					Expect(message.GetMessage()).To(Equal([]byte("my message")))
					Expect(message.GetMessageType()).To(Equal(events.LogMessage_ERR))
				})

				It("emits a log with app info", func() {
					client.EmitLog("my message",
						loggregator_v2.WithAppInfo("app-id", "source-type", "source-instance"),
					)

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetEventType()).To(Equal(events.Envelope_LogMessage))

					message := env.GetLogMessage()
					Expect(message.GetAppId()).To(Equal("app-id"))
					Expect(message.GetSourceType()).To(Equal("source-type"))
					Expect(message.GetSourceInstance()).To(Equal("source-instance"))
				})

				It("emits a log to stdout", func() {
					client.EmitLog("my message",
						loggregator_v2.WithStdout(),
					)

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetEventType()).To(Equal(events.Envelope_LogMessage))

					message := env.GetLogMessage()
					Expect(message.GetMessageType()).To(Equal(events.LogMessage_OUT))
				})
			})

			Describe("EmitCounter", func() {
				It("emits a counter", func() {
					client.EmitCounter("a-name")

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetEventType()).To(Equal(events.Envelope_CounterEvent))
					Expect(env.GetOrigin()).To(Equal("my-origin"))
					Expect(env.GetTimestamp()).To(BeNumerically("~", time.Now().UnixNano(), time.Second))

					counter := env.GetCounterEvent()
					Expect(counter.GetDelta()).To(Equal(uint64(1)))
				})

				It("emits a counter with a delta", func() {
					client.EmitCounter("a-name", loggregator_v2.WithDelta(404))

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetEventType()).To(Equal(events.Envelope_CounterEvent))

					counter := env.GetCounterEvent()
					Expect(counter.GetDelta()).To(Equal(uint64(404)))
				})
			})

			Describe("EmitGauge", func() {
				It("does not emit an empty gauge", func() {
					client.EmitGauge()

					Expect(spyEmitter.emittedEnvelopes).ToNot(Receive())
				})

				It("emits a gauge with one metric", func() {
					tags := map[string]string{
						"deployment": "a-deployment",
					}
					client.EmitGauge(
						loggregator_v2.WithGaugeValue("gauge-name", 123.45, "nanofortnights"),
						loggregator_v2.WithEnvelopeTags(tags),
						loggregator_v2.WithEnvelopeTag("other-tag", "some-value"),
					)

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetEventType()).To(Equal(events.Envelope_ValueMetric))
					Expect(env.GetOrigin()).To(Equal("my-origin"))
					Expect(env.GetTimestamp()).To(BeNumerically("~", time.Now().UnixNano(), time.Second))

					Expect(env.Tags).To(HaveKeyWithValue("deployment", "a-deployment"))
					Expect(env.Tags).To(HaveKeyWithValue("other-tag", "some-value"))

					gauge := env.GetValueMetric()
					Expect(gauge.GetName()).To(Equal("gauge-name"))
					Expect(gauge.GetValue()).To(Equal(123.45))
					Expect(gauge.GetUnit()).To(Equal("nanofortnights"))
				})

				It("emits envelopes with multiple metrics", func() {
					client.EmitGauge(
						loggregator_v2.WithGaugeValue("gauge-1", 123.45, "nanofortnights"),
						loggregator_v2.WithGaugeValue("gauge-2", 123.45, "nanofortnights"),
						loggregator_v2.WithGaugeValue("gauge-3", 123.45, "nanofortnights"),
					)

					Expect(spyEmitter.emittedEnvelopes).To(HaveLen(3))
				})

				It("emits envelopes with tags", func() {
					client.EmitGauge(
						loggregator_v2.WithGaugeValue("gauge-name", 123.45, "nanofortnights"),
						loggregator_v2.WithEnvelopeTags(map[string]string{
							"tag-1": "value-1",
							"tag-2": "value-2",
						}),
					)

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
					Expect(env.GetTags()).To(Equal(map[string]string{
						"tag-1": "value-1",
						"tag-2": "value-2",
					}))
				})

				It("emits envelopes with app info as a tag", func() {
					client.EmitGauge(
						loggregator_v2.WithGaugeValue("gauge-name", 123.45, "nanofortnights"),
						loggregator_v2.WithGaugeAppInfo("app-id", 123),
					)

					var env *events.Envelope
					Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))

					Expect(env.GetTags()).To(Equal(map[string]string{
						"source_id":   "app-id",
						"instance_id": "123",
					}))
				})

				Context("with IngressOptions", func() {
					BeforeEach(func() {
						client, _ = v1.NewClient(
							v1.WithTag("string-tag-name", "string-tag-value"),
						)
					})

					It("adds tags to logs", func() {
						client.EmitLog("a message")

						var env *events.Envelope
						Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))

						Expect(env.GetTags()).To(Equal(map[string]string{
							"string-tag-name": "string-tag-value",
						}))
					})

					It("adds tags to counters", func() {
						client.EmitCounter("counter-name")

						var env *events.Envelope
						Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))

						Expect(env.GetTags()).To(Equal(map[string]string{
							"string-tag-name": "string-tag-value",
						}))
					})

					It("adds tags to gauges", func() {
						client.EmitGauge(
							loggregator_v2.WithGaugeValue("gauge-name", 1.1, "dollars"),
							loggregator_v2.WithEnvelopeTags(map[string]string{
								"gauge-tag-name": "gauge-tag-value",
							}),
						)

						var env *events.Envelope
						Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))

						Expect(env.GetTags()).To(Equal(map[string]string{
							"string-tag-name": "string-tag-value",
							"gauge-tag-name":  "gauge-tag-value",
						}))
					})
				})

				Context("when the envelope should be promoted to a ContainerMetric", func() {
					It("promotes the envelope", func() {
						client.EmitGauge(
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
							loggregator_v2.WithGaugeAppInfo("some-app-id", 123),
						)

						var env *events.Envelope
						Expect(spyEmitter.emittedEnvelopes).To(HaveLen(1))
						Expect(spyEmitter.emittedEnvelopes).To(Receive(&env))
						Expect(env.GetContainerMetric()).ToNot(BeNil())
						Expect(env.GetContainerMetric().GetInstanceIndex()).To(Equal(int32(123)))
						Expect(env.GetContainerMetric().GetCpuPercentage()).To(Equal(float64(2)))
						Expect(env.GetContainerMetric().GetMemoryBytes()).To(Equal(uint64(3)))
						Expect(env.GetContainerMetric().GetDiskBytes()).To(Equal(uint64(4)))
						Expect(env.GetContainerMetric().GetMemoryBytesQuota()).To(Equal(uint64(5)))
						Expect(env.GetContainerMetric().GetDiskBytesQuota()).To(Equal(uint64(6)))
						Expect(env.GetContainerMetric().GetApplicationId()).To(Equal("some-app-id"))
					})

					It("does not promote the envelope if missing required name/value", func() {
						client.EmitGauge(
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							// Missing Disk Quota
							loggregator_v2.WithGaugeAppInfo("some-app-id", 123),
						)

						// It will not promote the envelope, and therefore
						// emit each one individually.
						Expect(spyEmitter.emittedEnvelopes).To(HaveLen(4))
					})

					DescribeTable("does not promote the envelope if a typo exists in one of the required metric names",
						func(opts []loggregator.EmitGaugeOption) {
							opts = append(opts, loggregator_v2.WithGaugeAppInfo("some-app-id", 123))

							client.EmitGauge(opts...)

							// It will not promote the envelope, and therefore
							// emit each one individually.
							Expect(spyEmitter.emittedEnvelopes).To(HaveLen(5))
						},
						Entry("cpu misspelled", []loggregator.EmitGaugeOption{
							loggregator_v2.WithGaugeValue("ccpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
						}),
						Entry("memory misspelled", []loggregator.EmitGaugeOption{
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("mmemory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
						}),
						Entry("disk misspelled", []loggregator.EmitGaugeOption{
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("ddisk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
						}),
						Entry("memory_quota misspelled", []loggregator.EmitGaugeOption{
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("mmemory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
						}),
						Entry("disk_quota misspelled", []loggregator.EmitGaugeOption{
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("ddisk_quota", 6, "bytes"),
						}),
					)

					It("does not promote the envelope if there are any extra name/value pairs", func() {
						client.EmitGauge(
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
							loggregator_v2.WithGaugeValue("extra", 9999, "bytes"),
							loggregator_v2.WithGaugeAppInfo("some-app-id", 123),
						)

						// It will not promote the envelope, and therefore
						// emit each one individually.
						Expect(spyEmitter.emittedEnvelopes).To(HaveLen(6))
					})

					It("does not promote the envelope if 'source_id' tag is missing", func() {
						client.EmitGauge(
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
							//	 Missing App Info
						)

						// It will not promote the envelope, and therefore
						// emit each one individually.
						Expect(spyEmitter.emittedEnvelopes).To(HaveLen(5))
					})

					It("does not promote the envelope if 'instance_id' tag is invalid", func() {
						client.EmitGauge(
							loggregator_v2.WithGaugeValue("cpu", 2, "percentage"),
							loggregator_v2.WithGaugeValue("memory", 3, "bytes"),
							loggregator_v2.WithGaugeValue("disk", 4, "bytes"),
							loggregator_v2.WithGaugeValue("memory_quota", 5, "bytes"),
							loggregator_v2.WithGaugeValue("disk_quota", 6, "bytes"),
							loggregator_v2.WithGaugeAppInfo("some-app-id", 123),

							// Squash previously set instance_id tag
							loggregator_v2.WithEnvelopeTag("instance_id", "invalid"),
						)

						// It will not promote the envelope, and therefore
						// emit each one individually.
						Expect(spyEmitter.emittedEnvelopes).To(HaveLen(5))
					})
				})
			})
		})

		// These tests are ensure the v1 client and v2 client both conform to the
		// v2 clients interface. If they do not these tests will cause a failure
		// to compile.
		It("conforms to the v2 interface", func() {
			type V2Interface interface {
				EmitLog(message string, opts ...loggregator_v2.EmitLogOption)
				EmitGauge(opts ...loggregator_v2.EmitGaugeOption)
				EmitCounter(name string, opts ...loggregator_v2.EmitCounterOption)
			}

			By("ensuring that the v2 ingress client conforms to v2 interface")
			var _ V2Interface = &loggregator_v2.IngressClient{}

			By("ensuring that the v1 client conforms to v2 interface")
			var _ V2Interface = &v1.Client{}
		})
	})
})

type SpyEventEmitter struct {
	emittedEnvelopes chan *events.Envelope
	origin           string
}

func NewSpyEventEmitter(origin string) *SpyEventEmitter {
	return &SpyEventEmitter{
		emittedEnvelopes: make(chan *events.Envelope, 100),
		origin:           origin,
	}
}

func (s *SpyEventEmitter) Emit(e events.Event) error {
	return nil
}

func (s *SpyEventEmitter) EmitEnvelope(envelope *events.Envelope) error {
	s.emittedEnvelopes <- envelope
	return nil
}

func (s *SpyEventEmitter) Origin() string {
	return s.origin
}
