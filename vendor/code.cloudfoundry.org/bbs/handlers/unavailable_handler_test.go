package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Unavailable Handler", func() {
	var (
		fakeServer   *ghttp.Server
		handler      *handlers.UnavailableHandler
		serviceReady chan struct{}

		request *http.Request
	)

	BeforeEach(func() {
		serviceReady = make(chan struct{})

		fakeServer = ghttp.NewServer()
		handler = handlers.NewUnavailableHandler(fakeServer, serviceReady)

		var err error
		request, err = http.NewRequest("GET", "/test", nil)
		Expect(err).NotTo(HaveOccurred())

		fakeServer.RouteToHandler("GET", "/test", ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/test"),
			ghttp.RespondWith(200, nil, nil),
		))
	})

	verifyResponse := func(expectedStatus int, handler *handlers.UnavailableHandler) {
		responseRecorder := httptest.NewRecorder()
		handler.ServeHTTP(responseRecorder, request)
		Expect(responseRecorder.Code).To(Equal(expectedStatus))
	}

	It("responds with 503 until the service is ready", func() {
		verifyResponse(http.StatusServiceUnavailable, handler)
		verifyResponse(http.StatusServiceUnavailable, handler)

		close(serviceReady)

		verifyResponse(http.StatusOK, handler)
		verifyResponse(http.StatusOK, handler)
	})
})
