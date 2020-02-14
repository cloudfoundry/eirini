package stager_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/opi/opifakes"
	. "code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/eirini/stager/stagerfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stager", func() {

	var (
		stager           eirini.Stager
		taskDesirer      *opifakes.FakeTaskDesirer
		stagingCompleter *stagerfakes.FakeStagingCompleter
		err              error
	)

	BeforeEach(func() {
		taskDesirer = new(opifakes.FakeTaskDesirer)

		logger := lagertest.NewTestLogger("test")
		config := &eirini.StagerConfig{
			EiriniAddress:   "http://opi.cf.internal",
			DownloaderImage: "eirini/recipe-downloader:tagged",
			UploaderImage:   "eirini/recipe-uploader:tagged",
			ExecutorImage:   "eirini/recipe-runner:tagged",
		}

		stagingCompleter = new(stagerfakes.FakeStagingCompleter)

		stager = &Stager{
			Desirer:          taskDesirer,
			StagingCompleter: stagingCompleter,
			Config:           config,
			Logger:           logger,
		}
	})

	Context("When staging", func() {
		var (
			stagingGUID string
			request     cf.StagingRequest
		)

		BeforeEach(func() {
			stagingGUID = "staging-id-123"

			request = cf.StagingRequest{
				AppGUID:   "our-app-id",
				AppName:   "our-app",
				OrgName:   "our-org",
				SpaceName: "our-space",
				OrgGUID:   "our-org-id",
				SpaceGUID: "our-space-id",
				Environment: []cf.EnvironmentVariable{
					{Name: "HOWARD", Value: "the alien"},
					{Name: eirini.EnvAppID, Value: "should be ignored"},
					{Name: eirini.EnvBuildpacks, Value: "should be ignored"},
					{Name: eirini.EnvDownloadURL, Value: "should be ignored"},
					{Name: eirini.EnvStagingGUID, Value: "should be ignored"},
					{Name: eirini.EnvEiriniAddress, Value: "should be ignored"},
					{Name: eirini.EnvCompletionCallback, Value: "should be ignored"},
					{Name: eirini.EnvDropletUploadURL, Value: "should be ignored"},
				},
				LifecycleData: &cf.StagingBuildpackLifecycle{
					AppBitsDownloadURI: "example.com/download",
					DropletUploadURI:   "example.com/upload",
					Buildpacks: []cf.Buildpack{
						{
							Name:       "go_buildpack",
							Key:        "1234eeff",
							URL:        "example.com/build/pack",
							SkipDetect: true,
						},
					},
				},
				CompletionCallback: "example.com/call/me/maybe",
				MemoryMB:           1234,
				DiskMB:             4567,
				CPUWeight:          49,
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
			Expect(task).To(Equal(&opi.StagingTask{
				DownloaderImage: "eirini/recipe-downloader:tagged",
				UploaderImage:   "eirini/recipe-uploader:tagged",
				ExecutorImage:   "eirini/recipe-runner:tagged",
				StagingGUID:     stagingGUID,
				Task: &opi.Task{
					AppName:   "our-app",
					AppGUID:   "our-app-id",
					OrgName:   "our-org",
					SpaceName: "our-space",
					OrgGUID:   "our-org-id",
					SpaceGUID: "our-space-id",
					Env: map[string]string{
						"HOWARD":                     "the alien",
						eirini.EnvDownloadURL:        "example.com/download",
						eirini.EnvDropletUploadURL:   "example.com/upload",
						eirini.EnvAppID:              request.AppGUID,
						eirini.EnvStagingGUID:        stagingGUID,
						eirini.EnvCompletionCallback: request.CompletionCallback,
						eirini.EnvBuildpacks:         `[{"name":"go_buildpack","key":"1234eeff","url":"example.com/build/pack","skip_detect":true}]`,
						eirini.EnvEiriniAddress:      "http://opi.cf.internal",
					},
					MemoryMB:  1234,
					DiskMB:    4567,
					CPUWeight: 49,
				},
			}))
		})

		Context("and desiring the task fails", func() {

			BeforeEach(func() {
				taskDesirer.DesireStagingReturns(errors.New("woopsie"))
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

		})

		Context("and there are no resource limitations", func() {
			BeforeEach(func() {
				request.MemoryMB = 0
				request.DiskMB = 0
				request.CPUWeight = 0
			})

			It("should set default values", func() {
				Expect(taskDesirer.DesireStagingCallCount()).To(Equal(1))
				task := taskDesirer.DesireStagingArgsForCall(0)
				Expect(task.MemoryMB).To(Equal(int64(200)))
				Expect(task.DiskMB).To(Equal(int64(500)))
				Expect(task.CPUWeight).To(Equal(uint8(50)))
			})
		})
	})

	Context("When completing staging", func() {

		var (
			task *models.TaskCallbackResponse
		)

		BeforeEach(func() {
			annotation := `{"completion_callback": "some-cc-endpoint.io/call/me/maybe"}`

			task = &models.TaskCallbackResponse{
				TaskGuid:      "our-task-guid",
				Failed:        false,
				FailureReason: "",
				Result:        `{"very": "good"}`,
				Annotation:    annotation,
				CreatedAt:     123456123,
			}
		})

		JustBeforeEach(func() {
			err = stager.CompleteStaging(task)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should complete staging", func() {
			Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))
			Expect(stagingCompleter.CompleteStagingArgsForCall(0)).To(Equal(task))
		})

		It("should delete the task", func() {
			Expect(taskDesirer.DeleteCallCount()).To(Equal(1))

			taskName := taskDesirer.DeleteArgsForCall(0)
			Expect(taskName).To(Equal(task.TaskGuid))
		})

		Context("and the staging completer fails", func() {
			BeforeEach(func() {
				stagingCompleter.CompleteStagingReturns(errors.New("complete boom"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("complete boom"))
			})

			It("should delete the task", func() {
				Expect(taskDesirer.DeleteCallCount()).To(Equal(1))

				taskName := taskDesirer.DeleteArgsForCall(0)
				Expect(taskName).To(Equal(task.TaskGuid))
			})
		})

		Context("and the task deletion fails", func() {
			BeforeEach(func() {
				taskDesirer.DeleteReturns(errors.New("delete boom"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("delete boom"))
			})
		})
	})
})
