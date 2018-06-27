package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Handler", func() {

	var (
		ts            *httptest.Server
		client        *http.Client
		bifrost       *eirinifakes.FakeBifrost
		handlerClient http.Handler
	)

	BeforeEach(func() {
		client = &http.Client{}
		bifrost = new(eirinifakes.FakeBifrost)
		lager := lagertest.NewTestLogger("handler-test")
		handlerClient = New(bifrost, lager)
	})

	JustBeforeEach(func() {
		ts = httptest.NewServer(handlerClient)
	})

	Context("Routes", func() {

		var (
			method         string
			path           string
			body           string
			expectedStatus int
		)

		assertEndpoint := func() {
			req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).ToNot(HaveOccurred())
			res, err := client.Do(req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.StatusCode).To(Equal(expectedStatus))

		}
		Context("PUT /apps/:process_guid", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/apps/myguid"
				body = `{"process_guid": "myguid", "num_instances": 5}`
				expectedStatus = http.StatusAccepted
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps", func() {

			BeforeEach(func() {
				method = "GET"
				path = "/apps"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("POST /apps/:process_guid", func() {

			BeforeEach(func() {
				method = "POST"
				path = "/apps/myguid"
				body = `{"process_guid": "myguid", "update": {"instances": 5}}`
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps/:process_guid", func() {

			BeforeEach(func() {
				method = "GET"
				path = "/apps/myguid"
				expectedStatus = http.StatusOK

				bifrost.GetAppReturns(&models.DesiredLRP{})
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps/:process_guid/instances", func() {
			BeforeEach(func() {
				method = "GET"
				path = "/apps/my-guid/instances"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})

		})

		Context("PUT /apps/:process_guid/stop", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/apps/myguid/stop"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})
	})

})
