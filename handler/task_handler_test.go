package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/handler/handlerfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		body = ""
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
				"guid": "some-guid",
				"name": "task-name",
				"app_guid": "our-app-id",
				"org_name": "our-org-name",
				"org_guid": "our-org-guid",
				"space_name": "our-space-name",
				"space_guid": "our-space-guid",
				"namespace": "our-namespace",
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
				GUID:               "some-guid",
				Name:               "task-name",
				AppGUID:            "our-app-id",
				OrgName:            "our-org-name",
				OrgGUID:            "our-org-guid",
				SpaceName:          "our-space-name",
				SpaceGUID:          "our-space-guid",
				Namespace:          "our-namespace",
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

	Describe("Cancel", func() {
		BeforeEach(func() {
			method = "DELETE"
			path = "/tasks/guid_1234"
			body = ""
		})

		It("succeeds", func() {
			Expect(response.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("should not transfer the task", func() {
			Expect(taskBifrost.TransferTaskCallCount()).To(Equal(0))
		})

		It("cancels the task", func() {
			Expect(taskBifrost.CancelTaskCallCount()).To(Equal(1))
			Expect(taskBifrost.CancelTaskArgsForCall(0)).To(Equal("guid_1234"))
		})

		When("cancelling the task fails", func() {
			BeforeEach(func() {
				taskBifrost.CancelTaskReturns(errors.New("BOOM"))
			})

			It("returns 500 status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Get", func() {
		BeforeEach(func() {
			method = "GET"
			path = "/tasks/guid_1234"
			body = ""

			taskBifrost.GetTaskReturns(cf.TaskResponse{
				GUID: "guid_1234",
			}, nil)
		})

		It("retrives a task", func() {
			Expect(taskBifrost.GetTaskCallCount()).To(Equal(1))
			actualGUID := taskBifrost.GetTaskArgsForCall(0)
			Expect(actualGUID).To(Equal("guid_1234"))

			var taskResponse cf.TaskResponse
			err := json.NewDecoder(response.Body).Decode(&taskResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(taskResponse.GUID).To(Equal("guid_1234"))
		})

		When("getting the task fails", func() {
			BeforeEach(func() {
				taskBifrost.GetTaskReturns(cf.TaskResponse{}, errors.New("task-error"))
			})

			It("returns a 500 status", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("List", func() {
		BeforeEach(func() {
			method = "GET"
			path = "/tasks"
			body = ""

			taskBifrost.ListTasksReturns([]cf.TaskResponse{{
				GUID: "guid_1234",
			}}, nil)
		})

		It("lists tasks", func() {
			Expect(taskBifrost.ListTasksCallCount()).To(Equal(1))

			var taskResponse []cf.TaskResponse
			err := json.NewDecoder(response.Body).Decode(&taskResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(taskResponse).To(HaveLen(1))
			Expect(taskResponse[0].GUID).To(Equal("guid_1234"))
		})

		When("listing tasks fails", func() {
			BeforeEach(func() {
				taskBifrost.ListTasksReturns(nil, errors.New("task-error"))
			})

			It("returns a 500 status", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
