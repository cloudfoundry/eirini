package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
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
			method = "PUT"
			path = "/stage/guid_1234"
			body = `{
				"app_id": "our-app-id",
				"file_descriptors": 2,
				"memory_mb": 256,
				"disk_mb": 512,
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"egress_rules": [{"protocol": "http"}],
				"timeout": 4,
				"log_guid": "our-log-guid",
				"lifecycle": "the-cycle-of-life",
				"lifecycle_data": { "earth_state": "flat" },
				"completion_callback": "example.com/call/me/maybe",
				"isolation_segment": "my-life"
			}`
		})

		It("should return 202 Accepted code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("should stage using the staging client", func() {
			Expect(stagingClient.StageCallCount()).To(Equal(1))
			stagingGUID, stagingRequest := stagingClient.StageArgsForCall(0)
			lData := json.RawMessage(`{ "earth_state": "flat" }`)

			Expect(stagingGUID).To(Equal("guid_1234"))
			Expect(stagingRequest).To(Equal(cc_messages.StagingRequestFromCC{
				AppId:           "our-app-id",
				FileDescriptors: 2,
				MemoryMB:        256,
				DiskMB:          512,
				Environment: []*models.EnvironmentVariable{
					{Name: "HOWARD", Value: "the alien"},
				},
				EgressRules: []*models.SecurityGroupRule{
					{Protocol: "http"},
				},
				Timeout:            4,
				LogGuid:            "our-log-guid",
				Lifecycle:          "the-cycle-of-life",
				LifecycleData:      &lData,
				CompletionCallback: "example.com/call/me/maybe",
				IsolationSegment:   "my-life",
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
			method = "POST"
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
