package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"

	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/handler/handlerfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("TaskHandler", func() {
	var (
		ts          *httptest.Server
		logger      *lagertest.TestLogger
		taskBifrost *handlerfakes.FakeTaskBifrost

		response *http.Response
		body     string
		path     string
		method   string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		taskBifrost = new(handlerfakes.FakeTaskBifrost)

		method = "POST"
		path = "/tasks/guid_1234"
		body = `{
				"name": "task-name",
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"completion_callback": "example.com/call/me/maybe",
				"lifecycle": {
					"buildpack_lifecycle": {
						"droplet_guid": "some-guid",
						"droplet_hash": "some-hash",
						"start_command": "some command"
					}
				}
			}`
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		handler := New(nil, nil, nil, taskBifrost, logger)
		ts = httptest.NewServer(handler)
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())

		client := &http.Client{}
		response, err = client.Do(req)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		ts.Close()
	})

	Describe("Run", func() {

		BeforeEach(func() {
			method = "POST"
			path = "/tasks/guid_1234"
			body = `{
                "name": "task-name",
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"completion_callback": "example.com/call/me/maybe",
				"lifecycle": {
          "buildpack_lifecycle": {
						"droplet_guid": "some-guid",
						"droplet_hash": "some-hash",
					  "start_command": "some command"
					}
				}
			}`
		})

		It("should return 202 Accepted code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("should transfer the task", func() {
			Expect(taskBifrost.TransferTaskCallCount()).To(Equal(1))
			_, actualTaskGUID, actualTaskRequest := taskBifrost.TransferTaskArgsForCall(0)
			Expect(actualTaskGUID).To(Equal("guid_1234"))
			Expect(actualTaskRequest).To(Equal(cf.TaskRequest{
				Name:               "task-name",
				AppGUID:            "our-app-id",
				Environment:        []cf.EnvironmentVariable{{Name: "HOWARD", Value: "the alien"}},
				CompletionCallback: "example.com/call/me/maybe",
				Lifecycle: cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{
						DropletGUID:  "some-guid",
						DropletHash:  "some-hash",
						StartCommand: "some command",
					},
				},
			}))
		})

		When("transferring the task fails", func() {
			BeforeEach(func() {
				taskBifrost.TransferTaskReturns(errors.New("transfer-task-err"))
			})

			It("should return 500 Internal Server Error code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the request body cannot be unmarshalled", func() {
			BeforeEach(func() {
				body = "random stuff"
			})

			It("should return 400 Bad Request code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should not transfer the task", func() {
				Expect(taskBifrost.TransferTaskCallCount()).To(Equal(0))
			})
		})
	})

	Describe("TaskCompleted", func() {
		BeforeEach(func() {
			method = "PUT"
			path = "/tasks/guid_1234/completed"
			body = ""
		})

		It("succeeds", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should not transfer the task", func() {
			Expect(taskBifrost.TransferTaskCallCount()).To(Equal(0))
		})

		It("completes the task", func() {
			Expect(taskBifrost.CompleteTaskCallCount()).To(Equal(1))
			Expect(taskBifrost.CompleteTaskArgsForCall(0)).To(Equal("guid_1234"))
		})

		When("completing the task fails", func() {
			BeforeEach(func() {
				taskBifrost.CompleteTaskReturns(errors.New("BOOM"))
			})

			It("returns 500 status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
