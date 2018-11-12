package loggregator_v2_test

import (
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/rfc5424"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syslog", func() {
	It("converts an envelope to syslog bytes", func() {
		env := &loggregator_v2.Envelope{}
		env.SourceId = "some-source-id"
		env.InstanceId = "some-instance-id"
		env.Timestamp = int64(time.Hour)

		d, err := env.Syslog(
			loggregator_v2.WithSyslogHostname("some-hostname"),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(d[0]).To(Equal([]byte("<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - - \n")))
	})

	It("defaults priority to 14", func() {
		env := &loggregator_v2.Envelope{}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.Priority).To(BeEquivalentTo(14))
	})

	DescribeTable("the proper process id is set", func(instanceID string) {
		env := &loggregator_v2.Envelope{
			InstanceId: instanceID,
		}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.ProcessID).To(Equal(instanceID))
	},
		Entry("numeric instance id", "26"),
		Entry("string instance id", "some-string"),
		Entry("empty instance id", ""),
	)

	It("can override process id", func() {
		env := &loggregator_v2.Envelope{}

		d, err := env.Syslog(loggregator_v2.WithSyslogProcessID("some-id"))

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.ProcessID).To(Equal("some-id"))
	})

	It("defaults hostname to empty string", func() {
		env := &loggregator_v2.Envelope{}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.Hostname).To(BeEmpty())
	})

	It("can override hostname", func() {
		env := &loggregator_v2.Envelope{}

		d, err := env.Syslog(loggregator_v2.WithSyslogHostname("some-hostname"))

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.Hostname).To(Equal("some-hostname"))
	})

	It("defaults app name to source id", func() {
		env := &loggregator_v2.Envelope{
			SourceId: "source-id",
		}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.AppName).To(Equal("source-id"))
	})

	It("can override app name", func() {
		env := &loggregator_v2.Envelope{}

		d, err := env.Syslog(loggregator_v2.WithSyslogAppName("some-app-name"))

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.AppName).To(Equal("some-app-name"))
	})

	It("sets the timestamp", func() {
		env := &loggregator_v2.Envelope{
			Timestamp: 12345000,
		}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.Timestamp.UnixNano()).To(BeEquivalentTo(12345000))
	})

	It("defaults the message to \\n", func() {
		env := &loggregator_v2.Envelope{}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.Message).To(Equal([]byte("\n")))
	})

	It("sets the tags as part of structured data", func() {
		env := &loggregator_v2.Envelope{}
		env.Tags = map[string]string{
			"namespace":  "oratos",
			"cluster-id": "some-cluster-id",
		}

		d, err := env.Syslog()

		Expect(err).ToNot(HaveOccurred())
		var msg rfc5424.Message
		err = msg.UnmarshalBinary(d[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(msg.StructuredData).To(HaveLen(1))
		Expect(msg.StructuredData[0].ID).To(Equal("tags@47450"))
		Expect(msg.StructuredData[0].Parameters).To(ConsistOf(
			rfc5424.SDParam{Name: "namespace", Value: "oratos"},
			rfc5424.SDParam{Name: "cluster-id", Value: "some-cluster-id"},
		))
	})

	Describe("Log Message", func() {
		It("converts a log to syslog bytes", func() {
			env := buildLogEnvelope("test-message", loggregator_v2.Log_OUT)
			env.SourceId = "some-source-id"
			env.InstanceId = "some-instance-id"
			env.Timestamp = int64(time.Hour)

			d, err := env.Syslog(
				loggregator_v2.WithSyslogHostname("some-hostname"),
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(d[0]).To(Equal([]byte("<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - - test-message\n")))
		})

		DescribeTable("the proper message is set", func(message, expected string) {
			env := buildLogEnvelope(message, loggregator_v2.Log_OUT)

			d, err := env.Syslog()

			Expect(err).ToNot(HaveOccurred())
			var msg rfc5424.Message
			err = msg.UnmarshalBinary(d[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Message).To(Equal([]byte(expected)))
		},
			Entry("empty message", "", "\n"),
			Entry("basic message", "some-log-message", "some-log-message\n"),
			Entry("message with null chars", "some\x00message", "somemessage\n"),
			Entry("message with existing newline", "some-message\n", "some-message\n"),
		)

		DescribeTable("the proper priority is set", func(
			logType loggregator_v2.Log_Type,
			expectedPriority int,
			errored bool,
		) {
			env := buildLogEnvelope("", logType)

			d, err := env.Syslog()

			if errored {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			var msg rfc5424.Message
			err = msg.UnmarshalBinary(d[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Priority).To(BeEquivalentTo(expectedPriority))
		},
			Entry("stdout", loggregator_v2.Log_OUT, 14, false),
			Entry("stderr", loggregator_v2.Log_ERR, 11, false),
			Entry("undefined type", loggregator_v2.Log_Type(-20), 0, true),
		)

	})

	Describe("Gauge Messages", func() {
		It("converts a gauge to syslog bytes", func() {
			env := buildGaugeEnvelope()
			env.SourceId = "some-source-id"
			env.InstanceId = "some-instance-id"
			env.Timestamp = int64(time.Hour)
			env.Tags = map[string]string{
				"namespace": "test-ns",
			}

			d, err := env.Syslog(
				loggregator_v2.WithSyslogHostname("some-hostname"),
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(d).To(ConsistOf(
				[]byte(`<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - [tags@47450 namespace="test-ns"][gauge@47450 name="cpu" value="0.23" unit="percentage"] `+"\n"),
				[]byte(`<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - [tags@47450 namespace="test-ns"][gauge@47450 name="memory" value="5423" unit="bytes"] `+"\n"),
			))
		})

		It("sets gauge metrics as structured data", func() {
			env := buildGaugeEnvelope()

			data, err := env.Syslog()

			Expect(err).ToNot(HaveOccurred())
			var sds [][]rfc5424.StructuredData
			for _, d := range data {
				var msg rfc5424.Message
				err = msg.UnmarshalBinary(d)
				Expect(err).ToNot(HaveOccurred())
				sds = append(sds, msg.StructuredData)
			}

			Expect(sds).To(ConsistOf([][]rfc5424.StructuredData{
				{
					{
						ID: "gauge@47450",
						Parameters: []rfc5424.SDParam{
							{
								Name:  "name",
								Value: "cpu",
							},
							{
								Name:  "value",
								Value: "0.23",
							},
							{
								Name:  "unit",
								Value: "percentage",
							},
						},
					},
				},
				{
					{
						ID: "gauge@47450",
						Parameters: []rfc5424.SDParam{
							{
								Name:  "name",
								Value: "memory",
							},
							{
								Name:  "value",
								Value: "5423",
							},
							{
								Name:  "unit",
								Value: "bytes",
							},
						},
					},
				},
			}))
		})
	})

	Describe("Counter Messages", func() {
		It("converts a counter to syslog bytes", func() {
			env := buildCounterEnvelope()
			env.SourceId = "some-source-id"
			env.InstanceId = "some-instance-id"
			env.Timestamp = int64(time.Hour)

			d, err := env.Syslog(
				loggregator_v2.WithSyslogHostname("some-hostname"),
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(d[0]).To(Equal([]byte(`<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - [counter@47450 name="some-counter" total="99" delta="1"] ` + "\n")))
		})

		It("sets counter values as structured data", func() {
			env := buildCounterEnvelope()

			d, err := env.Syslog()

			var msg rfc5424.Message
			err = msg.UnmarshalBinary(d[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.StructuredData).To(Equal([]rfc5424.StructuredData{
				{
					ID: "counter@47450",
					Parameters: []rfc5424.SDParam{
						{
							Name:  "name",
							Value: "some-counter",
						},
						{
							Name:  "total",
							Value: "99",
						},
						{
							Name:  "delta",
							Value: "1",
						},
					},
				},
			}))
		})
	})

	Describe("Event Messages", func() {
		It("converts an event to syslog bytes", func() {
			env := buildEventEnvelope()
			env.SourceId = "some-source-id"
			env.InstanceId = "some-instance-id"
			env.Timestamp = int64(time.Hour)

			d, err := env.Syslog(
				loggregator_v2.WithSyslogHostname("some-hostname"),
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(d[0]).To(Equal([]byte("<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - - some-title: some-body\n")))
		})

		It("sets the message from title and body", func() {
			env := buildEventEnvelope()

			d, err := env.Syslog()

			var msg rfc5424.Message
			err = msg.UnmarshalBinary(d[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Message).To(Equal([]byte("some-title: some-body\n")))
		})
	})

	Describe("Timer Messages", func() {
		It("converts a timer to syslog bytes", func() {
			env := buildTimerEnvelope()
			env.SourceId = "some-source-id"
			env.InstanceId = "some-instance-id"
			env.Timestamp = int64(time.Hour)

			d, err := env.Syslog(
				loggregator_v2.WithSyslogHostname("some-hostname"),
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(d[0]).To(Equal([]byte(`<14>1 1970-01-01T01:00:00+00:00 some-hostname some-source-id some-instance-id - [timer@47450 name="some-timer" start="0" stop="10"] ` + "\n")))
		})

		It("sets timer values as structured data", func() {
			env := buildTimerEnvelope()

			d, err := env.Syslog()

			var msg rfc5424.Message
			err = msg.UnmarshalBinary(d[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.StructuredData).To(Equal([]rfc5424.StructuredData{
				{
					ID: "timer@47450",
					Parameters: []rfc5424.SDParam{
						{
							Name:  "name",
							Value: "some-timer",
						},
						{
							Name:  "start",
							Value: "0",
						},
						{
							Name:  "stop",
							Value: "10",
						},
					},
				},
			}))
		})
	})

	DescribeTable("returns an error when envelope can not be converted", func(
		f func() *loggregator_v2.Envelope,
	) {
		v, err := f().Syslog()
		Expect(v).To(BeNil())
		Expect(err).To(HaveOccurred())
	},
		Entry("log message with non-printable source id", func() *loggregator_v2.Envelope {
			e := buildLogEnvelope("", loggregator_v2.Log_OUT)
			e.SourceId = "\x01"
			return e
		}),
		Entry("gauge message with source id that is > 48", func() *loggregator_v2.Envelope {
			e := buildGaugeEnvelope()
			e.SourceId = strings.Repeat("a", 49)
			return e
		}),
		Entry("counter message with non-printable instance id", func() *loggregator_v2.Envelope {
			e := buildCounterEnvelope()
			e.InstanceId = "\x01"
			return e
		}),
		Entry("event message with instance id that is > 128", func() *loggregator_v2.Envelope {
			e := buildEventEnvelope()
			e.InstanceId = strings.Repeat("a", 129)
			return e
		}),
		Entry("timer message with name that is not valid UTF-8", func() *loggregator_v2.Envelope {
			e := buildTimerEnvelope()
			e.GetTimer().Name = string([]byte{66, 250})
			return e
		}),
		Entry("bare envelope with non-printable source id", func() *loggregator_v2.Envelope {
			e := &loggregator_v2.Envelope{}
			e.SourceId = "\x01"
			return e
		}),
	)
})

func buildLogEnvelope(
	payload string,
	logType loggregator_v2.Log_Type,
) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(payload),
				Type:    logType,
			},
		},
	}
}

func buildGaugeEnvelope() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"cpu": &loggregator_v2.GaugeValue{
						Unit:  "percentage",
						Value: 0.23,
					},
					"memory": &loggregator_v2.GaugeValue{
						Unit:  "bytes",
						Value: 5423.0,
					},
				},
			},
		},
	}
}

func buildCounterEnvelope() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  "some-counter",
				Total: 99,
				Delta: 1,
			},
		},
	}
}

func buildEventEnvelope() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Event{
			Event: &loggregator_v2.Event{
				Title: "some-title",
				Body:  "some-body",
			},
		},
	}
}

func buildTimerEnvelope() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Timer{
			Timer: &loggregator_v2.Timer{
				Name:  "some-timer",
				Start: 0,
				Stop:  10,
			},
		},
	}
}
