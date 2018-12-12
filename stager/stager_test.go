package stager_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/opi/opifakes"
	. "code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Stager", func() {

	var (
		stager      Stager
		taskDesirer *opifakes.FakeTaskDesirer
		err         error
	)

	BeforeEach(func() {
		taskDesirer = new(opifakes.FakeTaskDesirer)

		logger := lagertest.NewTestLogger("test")
		config := &eirini.StagerConfig{
			CfUsername:    "admin",
			CfPassword:    "not1234567",
			APIAddress:    "api.bosh-lite.com",
			EiriniAddress: "http://opi.cf.internal",
			Image:         "eirini/recipe:tagged",
		}

		stager = Stager{
			Desirer:    taskDesirer,
			Config:     config,
			Logger:     logger,
			HTTPClient: &http.Client{},
		}
	})

	Context("When staging", func() {
		var (
			stagingGUID string
			request     cc_messages.StagingRequestFromCC
		)

		BeforeEach(func() {
			stagingGUID = "staging-id-123"

			lData := json.RawMessage(`{
				"app_bits_download_uri": "example.com/download",
				"droplet_upload_uri": "example.com/upload",
				"buildpacks": [
					{
						"name": "go_buildpack",
						"key": "1234eeff",
						"url": "example.com/build/pack",
						"skip_detect":true
					}
				]
			}`)
			request = cc_messages.StagingRequestFromCC{
				AppId:           "our-app-id",
				FileDescriptors: 2,
				MemoryMB:        256,
				DiskMB:          512,
				Environment: []*models.EnvironmentVariable{
					{Name: "HOWARD", Value: "the alien"},
					{Name: eirini.EnvAppID, Value: "should be ignored"},
					{Name: eirini.EnvAPIAddress, Value: "should be ignored"},
					{Name: eirini.EnvBuildpacks, Value: "should be ignored"},
					{Name: eirini.EnvCfPassword, Value: "should be ignored"},
					{Name: eirini.EnvCfUsername, Value: "should be ignored"},
					{Name: eirini.EnvDownloadURL, Value: "should be ignored"},
					{Name: eirini.EnvStagingGUID, Value: "should be ignored"},
					{Name: eirini.EnvEiriniAddress, Value: "should be ignored"},
					{Name: eirini.EnvCompletionCallback, Value: "should be ignored"},
					{Name: eirini.EnvDropletUploadURL, Value: "should be ignored"},
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
			}
		})

		JustBeforeEach(func() {
			err = stager.Stage(stagingGUID, request)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should desire a converted task without overriding eirini env variables", func() {
			Expect(taskDesirer.DesireStagingCallCount()).To(Equal(1))
			task := taskDesirer.DesireStagingArgsForCall(0)
			Expect(task).To(Equal(&opi.Task{
				Image: "eirini/recipe:tagged",
				Env: map[string]string{
					"HOWARD":                     "the alien",
					eirini.EnvDownloadURL:        "example.com/download",
					eirini.EnvDropletUploadURL:   "example.com/upload",
					eirini.EnvAppID:              request.LogGuid,
					eirini.EnvStagingGUID:        stagingGUID,
					eirini.EnvCompletionCallback: request.CompletionCallback,
					eirini.EnvBuildpacks:         `[{"name":"go_buildpack","key":"1234eeff","url":"example.com/build/pack","skip_detect":true}]`,
					eirini.EnvCfUsername:         "admin",
					eirini.EnvCfPassword:         "not1234567",
					eirini.EnvAPIAddress:         "api.bosh-lite.com",
					eirini.EnvEiriniAddress:      "http://opi.cf.internal",
				},
			}))
		})

		Context("and lifecycleData information is corrupted", func() {

			BeforeEach(func() {

				lData := json.RawMessage(`{
					"huh?what?":
				}`)

				request = cc_messages.StagingRequestFromCC{
					LifecycleData: &lData,
				}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())

			})

			It("should not desire the task", func() {
				Expect(taskDesirer.DesireStagingCallCount()).To(Equal(0))
			})

		})

		Context("and desiring the task fails", func() {

			BeforeEach(func() {
				taskDesirer.DesireStagingReturns(errors.New("woopsie"))
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Context("When completing staging", func() {

		var (
			server   *ghttp.Server
			task     *models.TaskCallbackResponse
			handlers []http.HandlerFunc
		)

		BeforeEach(func() {
			server = ghttp.NewServer()
			annotation := fmt.Sprintf(`{"completion_callback": "%s/call/me/maybe"}`, server.URL())

			task = &models.TaskCallbackResponse{
				TaskGuid:      "our-task-guid",
				Failed:        false,
				FailureReason: "",
				Result:        `{"very": "good"}`,
				Annotation:    annotation,
				CreatedAt:     123456123,
			}

			handlers = []http.HandlerFunc{
				ghttp.VerifyJSON(`{
					"result": {
						"very": "good"
					}
				}`),
			}
		})

		JustBeforeEach(func() {
			server.RouteToHandler("POST", "/call/me/maybe",
				ghttp.CombineHandlers(handlers...),
			)
			err = stager.CompleteStaging(task)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should post the response", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		It("should delete the task", func() {
			Expect(taskDesirer.DeleteCallCount()).To(Equal(1))

			taskName := taskDesirer.DeleteArgsForCall(0)
			Expect(taskName).To(Equal(task.TaskGuid))
		})

		Context("and the staging failed", func() {
			BeforeEach(func() {
				task.Failed = true
				task.FailureReason = "u broke my boy"
				task.Result = ""

				handlers = []http.HandlerFunc{
					ghttp.VerifyJSON(`{
						"error": {
							"id": "StagingError",
							"message": "u broke my boy"
						}
					}`),
				}
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should post the response", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("and the staging result is not a valid json", func() {
			BeforeEach(func() {
				task.Result = "{not valid json"
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not post the response", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("and the annotation is not a valid json", func() {
			BeforeEach(func() {
				task.Annotation = "{ !(valid json)"
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not post the response", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("and the callback response is an error", func() {
			BeforeEach(func() {
				handlers = []http.HandlerFunc{
					ghttp.RespondWith(http.StatusInternalServerError, ""),
				}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should post the response", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

	})
})
