package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/handler/handlerfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		ts                   *httptest.Server
		client               *http.Client
		lrpBifrost           *handlerfakes.FakeLRPBifrost
		dockerStagingBifrost *handlerfakes.FakeStagingBifrost
		taskBifrost          *handlerfakes.FakeTaskBifrost
		handlerClient        http.Handler
	)

	BeforeEach(func() {
		client = &http.Client{}
		lrpBifrost = new(handlerfakes.FakeLRPBifrost)
		dockerStagingBifrost = new(handlerfakes.FakeStagingBifrost)
		taskBifrost = new(handlerfakes.FakeTaskBifrost)

		lager := tests.NewTestLogger("handler-test")
		handlerClient = New(lrpBifrost, dockerStagingBifrost, taskBifrost, lager)
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
				method = http.MethodPut
				path = "/apps/myguid"
				expectedStatus = http.StatusAccepted
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps", func() {
			BeforeEach(func() {
				method = http.MethodGet
				path = "/apps"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("POST /apps/:process_guid", func() {
			BeforeEach(func() {
				method = http.MethodPost
				path = "/apps/myguid"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("GET /apps/:process_guid", func() {
			BeforeEach(func() {
				method = http.MethodGet
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
				method = http.MethodGet
				path = "/apps/my-guid/myversion/instances"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("PUT /apps/:process_guid/stop", func() {
			BeforeEach(func() {
				method = http.MethodPut
				path = "/apps/myguid/myversion/stop"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("PUT /apps/:process_guid/stop/:instance", func() {
			BeforeEach(func() {
				method = http.MethodPut
				path = "/apps/myguid/myversion/stop/1"
				expectedStatus = http.StatusOK
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("POST /stage/:staging_guid", func() {
			BeforeEach(func() {
				method = http.MethodPost
				path = "/stage/stage_123"
				body = `{"lifecycle" : { "docker_lifecycle": { } }}`
				expectedStatus = http.StatusAccepted
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("POST /tasks/:id", func() {
			BeforeEach(func() {
				method = http.MethodPost
				path = "/tasks/stage_123"
				expectedStatus = http.StatusAccepted
				body = `{"lifecycle" : {
				  "docker_lifecycle": {
					  "image": "eirini/dorini"
			      }
				}}`
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})

		Context("DELETE /tasks/:id", func() {
			BeforeEach(func() {
				method = http.MethodDelete
				path = "/tasks/task_123"
				expectedStatus = http.StatusNoContent
			})

			It("serves the endpoint", func() {
				assertEndpoint()
			})
		})
	})
})
