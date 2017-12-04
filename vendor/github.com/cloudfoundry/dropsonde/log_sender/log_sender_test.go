package log_sender_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/log_sender"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("LogSender", func() {
	var (
		mockBatcher *mockMetricBatcher
		emitter     *fake.FakeEventEmitter
		sender      *log_sender.LogSender
	)

	BeforeEach(func() {
		mockBatcher = newMockMetricBatcher()
		emitter = fake.NewFakeEventEmitter("test-origin")
		metrics.Initialize(nil, mockBatcher)
		sender = log_sender.NewLogSender(emitter, loggertesthelper.Logger())
	})

	AfterEach(func() {
		emitter.Close()
		for !emitter.IsClosed() {
			time.Sleep(10 * time.Millisecond)
		}
	})

	Describe("LogMessage", func() {
		It("sets the required properties", func() {
			msg := []byte("custom-log-message")
			msgType := events.LogMessage_OUT
			now := time.Now().UnixNano()

			err := sender.LogMessage(msg, msgType).Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			logMsg := emitter.GetEnvelopes()[0].LogMessage
			Expect(logMsg.GetMessage()).To(Equal(msg))
			Expect(logMsg.GetMessageType()).To(Equal(msgType))
			Expect(logMsg.GetTimestamp()).To(BeNumerically("~", now, time.Second))
		})

		It("allows for log message timestamp to be overwritten", func() {
			err := sender.LogMessage([]byte(""), events.LogMessage_OUT).
				SetTimestamp(-123456).
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			logMsg := emitter.GetEnvelopes()[0].LogMessage
			Expect(logMsg.GetTimestamp()).To(Equal(int64(-123456)))
		})

		Context("tags", func() {
			It("can add tags to AppLogs", func() {
				msg := []byte("custom-log-message")
				msgType := events.LogMessage_OUT
				err := sender.LogMessage(msg, msgType).
					SetTag("key", "value").
					SetTag("key2", "value2").
					Send()
				Expect(err).ToNot(HaveOccurred())

				Expect(emitter.GetEnvelopes()).To(HaveLen(1))
				envelope := emitter.GetEnvelopes()[0]
				Expect(envelope.GetTags()).To(HaveKeyWithValue("key", "value"))
				Expect(envelope.GetTags()).To(HaveKeyWithValue("key2", "value2"))
			})

			It("doesn't allow tag keys over 256 characters", func() {
				msg := []byte("custom-log-message")
				msgType := events.LogMessage_OUT

				tooLong := strings.Repeat("x", 257)
				err := sender.LogMessage(msg, msgType).
					SetTag(tooLong, "value").
					Send()
				Expect(err).To(HaveOccurred())
			})

			It("doesn't allow tag values over 256 characters", func() {
				msg := []byte("custom-log-message")
				msgType := events.LogMessage_OUT

				tooLong := strings.Repeat("x", 257)
				err := sender.LogMessage(msg, msgType).
					SetTag("key", tooLong).
					Send()
				Expect(err).To(HaveOccurred())
			})

			It("counts the number of runes in the key instead of bytes", func() {
				justRight := strings.Repeat("x", 255) + "Ω"
				msg := []byte("custom-log-message")
				msgType := events.LogMessage_OUT

				err := sender.LogMessage(msg, msgType).
					SetTag(justRight, "value").
					Send()
				Expect(err).ToNot(HaveOccurred())
			})

			It("counts the number of runes in the value instead of bytes", func() {
				justRight := strings.Repeat("x", 255) + "Ω"
				msg := []byte("custom-log-message")
				msgType := events.LogMessage_OUT

				err := sender.LogMessage(msg, msgType).
					SetTag("key", justRight).
					Send()
				Expect(err).ToNot(HaveOccurred())
			})

			It("doesn't allow more than 10 tags", func() {
				msg := []byte("custom-log-message")
				msgType := events.LogMessage_OUT

				c := sender.LogMessage(msg, msgType)
				for i := 0; i < 11; i++ {
					c = c.SetTag(fmt.Sprintf("key-%d", i), "value")
				}
				err := c.Send()
				Expect(err).To(HaveOccurred())
			})
		})

		It("sets envelope properties", func() {
			now := time.Now().UnixNano()
			err := sender.LogMessage([]byte(""), events.LogMessage_OUT).Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]
			Expect(envelope.GetOrigin()).To(Equal("test-origin"))
			Expect(envelope.GetEventType()).To(Equal(events.Envelope_LogMessage))
			Expect(envelope.GetTimestamp()).To(BeNumerically("~", now, time.Second))
		})

		It("allows for setting AppId on the LogMessage", func() {
			err := sender.LogMessage([]byte(""), events.LogMessage_OUT).
				SetAppId("test-app-id").
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			logMsg := emitter.GetEnvelopes()[0].LogMessage
			Expect(logMsg.GetAppId()).To(Equal("test-app-id"))
		})

		It("allows for setting SourceType on the LogMessage", func() {
			err := sender.LogMessage([]byte(""), events.LogMessage_OUT).
				SetSourceType("0").
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			logMsg := emitter.GetEnvelopes()[0].LogMessage
			Expect(logMsg.GetSourceType()).To(Equal("0"))
		})

		It("allows for setting SourceInstance on the LogMessage", func() {
			err := sender.LogMessage([]byte(""), events.LogMessage_OUT).
				SetSourceInstance("App").
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			logMsg := emitter.GetEnvelopes()[0].LogMessage
			Expect(logMsg.GetSourceInstance()).To(Equal("App"))
		})

		It("increments a counter for log messages sent to emitter", func() {
			c := sender.LogMessage([]byte(""), events.LogMessage_OUT)
			Expect(c.Send()).To(Succeed())
			Expect(c.Send()).To(Succeed())

			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("logSenderTotalMessagesRead"),
				With("logSenderTotalMessagesRead"),
			))
		})
	})

	Describe("SendAppLog", func() {
		It("sends a log message event to its emitter", func() {
			err := sender.SendAppLog("app-id", "custom-log-message", "App", "0")
			Expect(err).NotTo(HaveOccurred())

			Expect(emitter.GetMessages()).To(HaveLen(1))
			log := emitter.GetMessages()[0].Event.(*events.LogMessage)
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_OUT))
			Expect(log.GetMessage()).To(BeEquivalentTo("custom-log-message"))
			Expect(log.GetAppId()).To(Equal("app-id"))
			Expect(log.GetSourceType()).To(Equal("App"))
			Expect(log.GetSourceInstance()).To(Equal("0"))
			Expect(log.GetTimestamp()).ToNot(BeNil())
		})

		It("totals number of log messages sent to emitter", func() {
			sender.SendAppLog("app-id", "custom-log-message", "App", "0")
			sender.SendAppLog("app-id", "custom-log-message", "App", "0")

			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("logSenderTotalMessagesRead"),
				With("logSenderTotalMessagesRead"),
			))
		})

		It("emits counter metric on a timer", func() {
			sender.SendAppLog("app-id", "custom-log-message", "App", "0")
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("logSenderTotalMessagesRead"),
			))

			sender.SendAppLog("app-id", "custom-log-message", "App", "0")
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("logSenderTotalMessagesRead"),
			))
		})

		It("does not emit counter metrics when no logs are written", func() {
			Consistently(emitter.GetEvents, 1).Should(HaveLen(0))
		})
	})

	Describe("SendAppErrorLog", func() {
		It("sends a log error message event to its emitter", func() {
			err := sender.SendAppErrorLog("app-id", "custom-log-error-message", "App", "0")
			Expect(err).NotTo(HaveOccurred())

			Expect(emitter.GetMessages()).To(HaveLen(1))
			log := emitter.GetMessages()[0].Event.(*events.LogMessage)
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_ERR))
			Expect(log.GetMessage()).To(BeEquivalentTo("custom-log-error-message"))
			Expect(log.GetAppId()).To(Equal("app-id"))
			Expect(log.GetSourceType()).To(Equal("App"))
			Expect(log.GetSourceInstance()).To(Equal("0"))
			Expect(log.GetTimestamp()).ToNot(BeNil())
		})

		It("totals number of log messages sent to emitter", func() {
			sender.SendAppErrorLog("app-id", "custom-log-message", "App", "0")
			sender.SendAppErrorLog("app-id", "custom-log-message", "App", "0")

			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("logSenderTotalMessagesRead"),
				With("logSenderTotalMessagesRead"),
			))
		})
	})

	Describe("ScanLogStream", func() {
		It("sends lines from stream to emitter", func() {
			buf := bytes.NewBufferString("line 1\nline 2\n")

			sender.ScanLogStream("someId", "app", "0", buf)

			messages := emitter.GetMessages()
			Expect(messages).To(HaveLen(2))

			log := emitter.GetMessages()[0].Event.(*events.LogMessage)
			Expect(log.GetMessage()).To(BeEquivalentTo("line 1"))
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_OUT))
			Expect(log.GetAppId()).To(Equal("someId"))
			Expect(log.GetSourceType()).To(Equal("app"))
			Expect(log.GetSourceInstance()).To(Equal("0"))

			log = emitter.GetMessages()[1].Event.(*events.LogMessage)
			Expect(log.GetMessage()).To(BeEquivalentTo("line 2"))
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_OUT))
		})

		It("logs a message and returns on read errors", func() {
			var errReader fakeReader
			sender.ScanLogStream("someId", "app", "0", &errReader)

			messages := emitter.GetMessages()
			Expect(messages).To(HaveLen(1))

			log := emitter.GetMessages()[0].Event.(*events.LogMessage)
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_OUT))
			Expect(log.GetMessage()).To(BeEquivalentTo("one"))

			loggerMessage := loggertesthelper.TestLoggerSink.LogContents()
			Expect(loggerMessage).To(ContainSubstring("Read Error"))
		})

		It("stops when reader returns EOF", func() {
			reader := infiniteReader{
				stopChan: make(chan struct{}),
			}
			doneChan := make(chan struct{})

			go func() {
				sender.ScanLogStream("someId", "app", "0", reader)
				close(doneChan)
			}()
			go keepMockChansDrained(
				reader.stopChan,
				mockBatcher.BatchIncrementCounterCalled,
				mockBatcher.BatchIncrementCounterInput,
			)

			Eventually(func() int { return len(emitter.GetMessages()) }).Should(BeNumerically(">", 1))
			close(reader.stopChan)
			Eventually(doneChan).Should(BeClosed())
		})

		It("drops over-length messages and resumes scanning", func() {
			// Scanner can't handle tokens over 64K
			bigReader := strings.NewReader(strings.Repeat("x", 64*1024+1) + "\nsmall message\n")

			doneChan := make(chan struct{})
			go func() {
				sender.ScanLogStream("someId", "app", "0", bigReader)
				close(doneChan)
			}()

			Eventually(func() int { return len(emitter.GetMessages()) }).Should(BeNumerically(">=", 3))

			Eventually(doneChan).Should(BeClosed())

			messages := getLogMessages(emitter.GetMessages())

			Expect(messages[0]).To(ContainSubstring("Dropped log message: message too long (>64K without a newline)"))
			Expect(messages[1]).To(Equal("x"))
			Expect(messages[2]).To(Equal("small message"))
		})

		It("ignores empty lines", func() {
			reader := strings.NewReader("one\n\ntwo\n")

			sender.ScanLogStream("someId", "app", "0", reader)

			Expect(emitter.GetMessages()).To(HaveLen(2))
			messages := getLogMessages(emitter.GetMessages())

			Expect(messages[0]).To(Equal("one"))
			Expect(messages[1]).To(Equal("two"))
		})
	})

	Describe("ScanErrorLogStream", func() {

		It("sends lines from stream to emitter", func() {
			buf := bytes.NewBufferString("line 1\nline 2\n")

			sender.ScanErrorLogStream("someId", "app", "0", buf)

			messages := emitter.GetMessages()
			Expect(messages).To(HaveLen(2))

			log := emitter.GetMessages()[0].Event.(*events.LogMessage)
			Expect(log.GetMessage()).To(BeEquivalentTo("line 1"))
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_ERR))
			Expect(log.GetAppId()).To(Equal("someId"))
			Expect(log.GetSourceType()).To(Equal("app"))
			Expect(log.GetSourceInstance()).To(Equal("0"))

			log = emitter.GetMessages()[1].Event.(*events.LogMessage)
			Expect(log.GetMessage()).To(BeEquivalentTo("line 2"))
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_ERR))
		})

		It("logs a message and stops on read errors", func() {
			var errReader fakeReader
			sender.ScanErrorLogStream("someId", "app", "0", &errReader)

			messages := emitter.GetMessages()
			Expect(messages).To(HaveLen(1))

			log := emitter.GetMessages()[0].Event.(*events.LogMessage)
			Expect(log.GetMessageType()).To(Equal(events.LogMessage_ERR))
			Expect(log.GetMessage()).To(BeEquivalentTo("one"))

			loggerMessage := loggertesthelper.TestLoggerSink.LogContents()
			Expect(loggerMessage).To(ContainSubstring("Read Error"))
		})

		It("stops when reader returns EOF", func() {
			var reader infiniteReader
			reader.stopChan = make(chan struct{})
			doneChan := make(chan struct{})

			go func() {
				sender.ScanErrorLogStream("someId", "app", "0", reader)
				close(doneChan)
			}()
			go keepMockChansDrained(
				reader.stopChan,
				mockBatcher.BatchIncrementCounterCalled,
				mockBatcher.BatchIncrementCounterInput,
			)

			Eventually(func() int { return len(emitter.GetMessages()) }).Should(BeNumerically(">", 1))

			close(reader.stopChan)
			Eventually(doneChan).Should(BeClosed())
		})

		It("drops over-length messages and resumes scanning", func() {
			// Scanner can't handle tokens over 64K
			bigReader := strings.NewReader(strings.Repeat("x", 64*1024+1) + "\nsmall message\n")
			sender.ScanErrorLogStream("someId", "app", "0", bigReader)
			var lenMessages = func() int { return len(emitter.GetMessages()) }
			Eventually(lenMessages).Should(BeNumerically(">=", 3))

			messages := getLogMessages(emitter.GetMessages())

			Expect(messages[0]).To(ContainSubstring("Dropped log message: message too long (>64K without a newline)"))
			Expect(messages[1]).To(Equal("x"))
			Expect(messages[2]).To(Equal("small message"))
		})

		It("ignores empty lines", func() {
			reader := strings.NewReader("one\n \ntwo\n")

			sender.ScanErrorLogStream("someId", "app", "0", reader)

			Expect(emitter.GetMessages()).To(HaveLen(2))
			messages := getLogMessages(emitter.GetMessages())

			Expect(messages[0]).To(Equal("one"))
			Expect(messages[1]).To(Equal("two"))
		})
	})

	Context("when messages cannot be emitted", func() {
		BeforeEach(func() {
			emitter.ReturnError = errors.New("expected error")
		})

		Describe("SendAppLog", func() {
			It("sends an error when log messages cannot be emitted", func() {
				err := sender.SendAppLog("app-id", "custom-log-message", "App", "0")
				Expect(err).To(HaveOccurred())
			})

		})

		Describe("SendAppErrorLog", func() {
			It("sends an error when log error messages cannot be emitted", func() {
				err := sender.SendAppErrorLog("app-id", "custom-log-error-message", "App", "0")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
