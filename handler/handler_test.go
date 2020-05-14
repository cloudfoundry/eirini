package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/handler/handlerfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Handler", func() {

	var (
		ts                      *httptest.Server
		client                  *http.Client
		lrpBifrost              *handlerfakes.FakeLRPBifrost
		buildpackStagingBifrost *handlerfakes.FakeStagingBifrost
		dockerStagingBifrost    *handlerfakes.FakeStagingBifrost
		taskBifrost             *handlerfakes.FakeTaskBifrost
		handlerClient           http.Handler
	)

	BeforeEach(func() {
		client = &http.Client{}
		lrpBifrost = new(handlerfakes.FakeLRPBifrost)
		buildpackStagingBifrost = new(handlerfakes.FakeStagingBifrost)
		dockerStagingBifrost = new(handlerfakes.FakeStagingBifrost)
		taskBifrost = new(handlerfakes.FakeTaskBifrost)

		lager := lagertest.NewTestLogger("handler-test")
		handlerClient = New(lrpBifrost, buildpackStagingBifrost, dockerStagingBifrost, taskBifrost, lager)
	})

	JustBeforeEach(func() {
		ts = httptest.NewServer(handlerClient)
	})

	Context("Routes", func() {

		var (
			method         string
			path           string
			expectedStatus int
			body           string
		)

		assertEndpoint := func() {
			req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).ToNot(HaveOccurred())
			res, err := client.Do(req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.StatusCode).To(Equal(expectedStatus))
		}

		BeforeEach(func() {
			body = "{}"
		})

		Context("PUT /apps/:process_guid", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/apps/myguid"
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
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps/:process_guid", func() {

			BeforeEach(func() {
				method = "GET"
				path = "/apps/myguid/myversion"
				expectedStatus = http.StatusOK

				lrpBifrost.GetAppReturns(cf.DesiredLRP{}, nil)
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps/:process_guid/instances", func() {
			BeforeEach(func() {
				method = "GET"
				path = "/apps/my-guid/myversion/instances"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})

		})

		Context("PUT /apps/:process_guid/stop", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/apps/myguid/myversion/stop"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("PUT /apps/:process_guid/stop/:instance", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/apps/myguid/myversion/stop/1"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("POST /stage/:staging_guid", func() {

			BeforeEach(func() {
				method = "POST"
				path = "/stage/stage_123"
				expectedStatus = http.StatusAccepted
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("PUT /stage/:staging_guid/completed", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/stage/stage_123/completed"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("POST /tasks/:id", func() {

			BeforeEach(func() {
				method = "POST"
				path = "/tasks/stage_123"
				expectedStatus = http.StatusAccepted
				body = `{"lifecycle" : {
          "buildpack_lifecycle": {
					  "start_command": "cmd"
					}
				}}`

			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("PUT /tasks/:id/completed", func() {

			BeforeEach(func() {
				method = "PUT"
				path = "/tasks/task_123/completed"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})

		})
	})

})
