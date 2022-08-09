package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"

	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/handler/handlerfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StageHandler", func() {
	var (
		ts     *httptest.Server
		logger *tests.TestLogger

		dockerStagingClient *handlerfakes.FakeStagingBifrost
		bifrostTaskClient   *handlerfakes.FakeTaskBifrost
		response            *http.Response
		body                string
		path                string
		method              string
	)

	BeforeEach(func() {
		logger = tests.NewTestLogger("test")
		dockerStagingClient = new(handlerfakes.FakeStagingBifrost)
		bifrostTaskClient = new(handlerfakes.FakeTaskBifrost)
	})

	JustBeforeEach(func() {
		handler := New(nil, dockerStagingClient, bifrostTaskClient, logger)
		ts = httptest.NewServer(handler)
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())

		client := &http.Client{}
		response, err = client.Do(req)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When an app is submitted for staging", func() {
		BeforeEach(func() {
			method = http.MethodPost
			path = "/stage/guid_1234"
			body = `{
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"lifecycle": {
					"docker_lifecycle": {
						"image": "eirini/repo",
						"registry_username": "user",
						"registry_password": "pass"
					}
				},
				"completion_callback": "example.com/call/me/maybe"
			}`
		})

		It("should return 202 Accepted code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("should stage it using the staging client", func() {
			Expect(dockerStagingClient.TransferStagingCallCount()).To(Equal(1))
			_, stagingGUID, stagingRequest := dockerStagingClient.TransferStagingArgsForCall(0)

			Expect(stagingGUID).To(Equal("guid_1234"))
			Expect(stagingRequest).To(Equal(cf.StagingRequest{
				AppGUID: "our-app-id",
				Environment: []cf.EnvironmentVariable{
					{Name: "HOWARD", Value: "the alien"},
				},
				Lifecycle: cf.StagingLifecycle{
					DockerLifecycle: &cf.StagingDockerLifecycle{
						Image:            "eirini/repo",
						RegistryUsername: "user",
						RegistryPassword: "pass",
					},
				},
				CompletionCallback: "example.com/call/me/maybe",
			}))
		})

		Context("and the lifecycle type is unsupported", func() {
			BeforeEach(func() {
				body = `{
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"lifecycle": {
					"buildpack_lifecycle": {
						"app_bits_download_uri": "example.com/download",
						"droplet_upload_uri": "example.com/upload"
					}
				},
				"completion_callback": "example.com/call/me/maybe"
			}`
			})

			It("should return a 400 Bad Request status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should return the error in the response body", func() {
				bytes, _ := io.ReadAll(response.Body)
				stagingError := cf.Error{}
				err := json.Unmarshal(bytes, &stagingError)
				Expect(err).ToNot(HaveOccurred())
				Expect(stagingError.Message).To(ContainSubstring("docker is the only supported lifecycle"))
			})

			It("should not desire a task", func() {
				Expect(dockerStagingClient.TransferStagingCallCount()).To(Equal(0))
			})
		})

		Context("and the staging client fails", func() {
			BeforeEach(func() {
				dockerStagingClient.TransferStagingReturns(errors.New("underlying-err"))
			})

			It("should return a 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("should return a high level error in the response body", func() {
				bytes, _ := io.ReadAll(response.Body)
				stagingError := cf.Error{}
				err := json.Unmarshal(bytes, &stagingError)
				Expect(err).ToNot(HaveOccurred())
				Expect(stagingError.Message).To(Equal(`failed to stage task with guid "guid_1234": underlying-err`))
			})

			It("should log the underlying error", func() {
				Expect(logger.Logs()).NotTo(BeEmpty())
				Expect(logger.Logs()[0].Data).To(HaveKeyWithValue("error",
					`failed to stage task with guid "guid_1234": underlying-err`))
			})
		})

		Context("and the body is invalid", func() {
			BeforeEach(func() {
				body = "{ this json is invalid"
			})

			It("should return a 400 Bad Request status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should return the error in the response body", func() {
				bytes, _ := io.ReadAll(response.Body)
				stagingError := cf.Error{}
				err := json.Unmarshal(bytes, &stagingError)
				Expect(err).ToNot(HaveOccurred())
				Expect(stagingError.Message).ToNot(BeEmpty())
			})

			It("should not desire a task", func() {
				Expect(dockerStagingClient.TransferStagingCallCount()).To(Equal(0))
			})
		})
	})
})
