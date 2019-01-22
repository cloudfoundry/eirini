package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("StageHandler", func() {

	var (
		ts     *httptest.Server
		logger lager.Logger

		stagingClient *eirinifakes.FakeStager
		bifrost       *eirinifakes.FakeBifrost
		response      *http.Response
		body          string
		path          string
		method        string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		stagingClient = new(eirinifakes.FakeStager)
		bifrost = new(eirinifakes.FakeBifrost)
	})

	JustBeforeEach(func() {
		handler := New(bifrost, stagingClient, logger)
		ts = httptest.NewServer(handler)
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())

		client := &http.Client{}
		response, err = client.Do(req)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When an app is submitted for staging", func() {

		BeforeEach(func() {
			method = "POST"
			path = "/stage/guid_1234"
			body = `{
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"lifecycle_data": {
					"app_bits_download_uri": "example.com/download",
					"droplet_upload_uri": "example.com/upload",
					"buildpacks": []
				},
				"completion_callback": "example.com/call/me/maybe"
			}`
		})

		It("should return 202 Accepted code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("should stage using the staging client", func() {
			Expect(stagingClient.StageCallCount()).To(Equal(1))
			stagingGUID, stagingRequest := stagingClient.StageArgsForCall(0)

			Expect(stagingGUID).To(Equal("guid_1234"))
			Expect(stagingRequest).To(Equal(cf.StagingRequest{
				AppGUID: "our-app-id",
				Environment: []cf.EnvironmentVariable{
					{Name: "HOWARD", Value: "the alien"},
				},
				LifecycleData: cf.LifecycleData{
					AppBitsDownloadURI: "example.com/download",
					DropletUploadURI:   "example.com/upload",
					Buildpacks:         []cf.Buildpack{},
				},
				CompletionCallback: "example.com/call/me/maybe",
			}))
		})

		Context("and the body is invalid", func() {
			BeforeEach(func() {
				body = "{ this json is invalid"
			})

			It("should return a 400 Bad Request status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should not desire a task", func() {
				Expect(stagingClient.StageCallCount()).To(Equal(0))
			})
		})

		Context("and the staging client fails", func() {
			BeforeEach(func() {
				stagingClient.StageReturns(errors.New("pow"))
			})

			It("should return a 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Context("When app staging is completed", func() {
		BeforeEach(func() {
			method = "PUT"
			path = "/stage/staging_123523/completed"
			body = `{
				"task_guid": "our-task-guid",
				"failed": false,
				"failure_reason": "",
				"result": "very good",
				"annotation": "{\"lifecycle\": \"the-cycle-of-life\",\"completion_callback\": \"example.com/call/me/maybe\"}",
				"created_at": 123456123
			}`
		})

		It("should return a 200 OK status code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should submit the task callback response", func() {
			Expect(stagingClient.CompleteStagingCallCount()).To(Equal(1))
			task := stagingClient.CompleteStagingArgsForCall(0)
			Expect(task).To(Equal(&models.TaskCallbackResponse{
				TaskGuid:      "our-task-guid",
				Failed:        false,
				FailureReason: "",
				Result:        "very good",
				Annotation:    `{"lifecycle": "the-cycle-of-life","completion_callback": "example.com/call/me/maybe"}`,
				CreatedAt:     123456123,
			}))
		})

		Context("and submitting the task callback response fails", func() {
			BeforeEach(func() {
				stagingClient.CompleteStagingReturns(errors.New("boo"))
			})

			It("should return a 500 Internal Server Error response code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

		})
	})

})
