package middleware_test

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/handlers/middleware"
	"code.cloudfoundry.org/bbs/handlers/middleware/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Middleware", func() {
	Describe("RecordRequestCount", func() {
		var (
			handler http.HandlerFunc
			emitter *fakes.FakeEmitter
		)

		BeforeEach(func() {
			emitter = &fakes.FakeEmitter{}
			handler = func(w http.ResponseWriter, r *http.Request) { time.Sleep(10) }
			handler = middleware.RecordRequestCount(handler, emitter)
		})

		It("reports call count", func() {
			handler.ServeHTTP(nil, nil)
			handler.ServeHTTP(nil, nil)
			handler.ServeHTTP(nil, nil)

			Expect(emitter.IncrementCounterCallCount()).To(Equal(3))
		})
	})

	Describe("LogWrap", func() {
		var (
			logger              *lagertest.TestLogger
			loggableHandlerFunc middleware.LoggableHandlerFunc
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test-session")
			logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
			loggableHandlerFunc = func(logger lager.Logger, w http.ResponseWriter, r *http.Request) {
				logger = logger.Session("logger-group")
				logger.Info("written-in-loggable-handler")
			}
		})

		It("creates \"request\" session and passes it to LoggableHandlerFunc", func() {
			handler := middleware.LogWrap(logger, nil, loggableHandlerFunc)
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(nil, req)
			Expect(logger.Buffer()).To(gbytes.Say("test-session.request.serving"))
			Expect(logger.Buffer()).To(gbytes.Say("\"session\":\"1\""))
			Expect(logger.Buffer()).To(gbytes.Say("test-session.request.logger-group.written-in-loggable-handler"))
			Expect(logger.Buffer()).To(gbytes.Say("\"session\":\"1.1\""))
			Expect(logger.Buffer()).To(gbytes.Say("test-session.request.done"))
			Expect(logger.Buffer()).To(gbytes.Say("\"session\":\"1\""))
		})

		Context("with access loggger", func() {
			var accessLogger *lagertest.TestLogger

			BeforeEach(func() {
				accessLogger = lagertest.NewTestLogger("test-access-session")
				accessLogger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

				handler := middleware.LogWrap(logger, accessLogger, loggableHandlerFunc)
				req, err := http.NewRequest("GET", "http://example.com", nil)
				Expect(err).NotTo(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:8080"

				handler.ServeHTTP(nil, req)
			})

			It("creates \"request\" session and passes it to LoggableHandlerFunc", func() {
				Expect(logger.Buffer()).To(gbytes.Say("test-session.request.serving"))
				Expect(logger.Buffer()).To(gbytes.Say("\"session\":\"1\""))
				Expect(accessLogger.Buffer()).To(gbytes.Say("test-access-session.request.serving"))
				Expect(accessLogger.Buffer()).To(gbytes.Say("\"session\":\"1\""))

				Expect(logger.Buffer()).To(gbytes.Say("test-session.request.logger-group.written-in-loggable-handler"))
				Expect(logger.Buffer()).To(gbytes.Say("\"session\":\"1.1\""))

				Expect(accessLogger.Buffer()).To(gbytes.Say("test-access-session.request.done"))
				Expect(accessLogger.Buffer()).To(gbytes.Say("\"session\":\"1\""))
				Expect(logger.Buffer()).To(gbytes.Say("test-session.request.done"))
				Expect(logger.Buffer()).To(gbytes.Say("\"session\":\"1\""))
			})

			It("logs method, reqeust, ip, and port to serving and done logs", func() {
				Expect(logger.Buffer()).To(gbytes.Say("test-session.request.serving"))
				Expect(logger.Buffer()).To(gbytes.Say("method\":\"GET\""))
				Expect(logger.Buffer()).To(gbytes.Say("remote_addr\":\"127.0.0.1:8080\""))
				Expect(logger.Buffer()).To(gbytes.Say("request\":\"http://example.com\""))

				Expect(accessLogger.Buffer()).To(gbytes.Say("test-access-session.request.serving"))
				Expect(accessLogger.Buffer()).To(gbytes.Say("method\":\"GET\""))
				Expect(accessLogger.Buffer()).To(gbytes.Say("remote_addr\":\"127.0.0.1:8080\""))
				Expect(accessLogger.Buffer()).To(gbytes.Say("request\":\"http://example.com\""))

				Expect(logger.Buffer()).To(gbytes.Say("test-session.request.done"))
				Expect(logger.Buffer()).To(gbytes.Say("method\":\"GET\""))
				Expect(logger.Buffer()).To(gbytes.Say("remote_addr\":\"127.0.0.1:8080\""))
				Expect(logger.Buffer()).To(gbytes.Say("request\":\"http://example.com\""))

				Expect(accessLogger.Buffer()).To(gbytes.Say("test-access-session.request.done"))
				Expect(accessLogger.Buffer()).To(gbytes.Say("method\":\"GET\""))
				Expect(accessLogger.Buffer()).To(gbytes.Say("remote_addr\":\"127.0.0.1:8080\""))
				Expect(accessLogger.Buffer()).To(gbytes.Say("request\":\"http://example.com\""))

			})
		})
	})
})
