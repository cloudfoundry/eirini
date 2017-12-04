package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/bulk/fakes"
	"code.cloudfoundry.org/nsync/handlers"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/runtimeschema/cc_messages"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DesireTaskHandler", func() {
	var (
		logger           *lagertest.TestLogger
		fakeBBSClient    *fake_bbs.FakeClient
		buildpackBuilder *fakes.FakeRecipeBuilder
		taskRequest      cc_messages.TaskRequestFromCC

		request          *http.Request
		responseRecorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		var err error

		logger = lagertest.NewTestLogger("test")
		fakeBBSClient = new(fake_bbs.FakeClient)
		buildpackBuilder = new(fakes.FakeRecipeBuilder)

		taskRequest = cc_messages.TaskRequestFromCC{
			TaskGuid:  "the-task-guid",
			LogGuid:   "some-log-guid",
			MemoryMb:  128,
			DiskMb:    512,
			Lifecycle: "test",
			EnvironmentVariables: []*models.EnvironmentVariable{
				{Name: "foo", Value: "bar"},
				{Name: "VCAP_APPLICATION", Value: "{\"application_name\":\"my-app\"}"},
			},
			DropletUri:            "http://the-droplet.uri.com",
			RootFs:                "http://docker-image.com",
			CompletionCallbackUrl: "http://api.cc.com/v1/tasks/complete",
			Command:               "the-start-command",
		}

		responseRecorder = httptest.NewRecorder()

		request, err = http.NewRequest("POST", "", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		if request.Body == nil {
			jsonBytes, err := json.Marshal(&taskRequest)
			Expect(err).NotTo(HaveOccurred())
			reader := bytes.NewReader(jsonBytes)

			request.Body = ioutil.NopCloser(reader)
		}

		handler := handlers.NewTaskHandler(logger, fakeBBSClient, map[string]recipebuilder.RecipeBuilder{
			"test": buildpackBuilder,
		})
		handler.DesireTask(responseRecorder, request)
	})

	Context("when the task does not exist", func() {
		var newlyDesiredTask *models.TaskDefinition

		BeforeEach(func() {
			newlyDesiredTask = &models.TaskDefinition{
				LogGuid:  "some-log-guid",
				MemoryMb: 128,
				DiskMb:   512,
				EnvironmentVariables: []*models.EnvironmentVariable{
					{Name: "foo", Value: "bar"},
					{Name: "VCAP_APPLICATION", Value: "{\"application_name\":\"my-app\"}"},
				},
				RootFs:                "http://docker-image.com",
				CompletionCallbackUrl: "http://api.cc.com/v1/tasks/complete",
				Action: models.WrapAction(models.Serial(
					&models.DownloadAction{
						From:     taskRequest.DropletUri,
						To:       ".",
						CacheKey: "",
						User:     "vcap",
					},
					&models.RunAction{},
				)),
			}

			buildpackBuilder.BuildTaskReturns(newlyDesiredTask, nil)
		})

		It("logs the incoming and outgoing request", func() {
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("serving"))
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("desiring-task"))
		})

		It("creates the task", func() {
			Expect(buildpackBuilder.BuildTaskCallCount()).To(Equal(1))
			Expect(buildpackBuilder.BuildTaskArgsForCall(0)).To(Equal(&taskRequest))
			Expect(fakeBBSClient.DesireTaskCallCount()).To(Equal(1))

			_, guid, domain, taskDefinition := fakeBBSClient.DesireTaskArgsForCall(0)
			Expect(guid).To(Equal("the-task-guid"))
			Expect(domain).To(Equal("cf-tasks"))
			Expect(taskDefinition).To(Equal(newlyDesiredTask))
		})

		It("responds with 202 Accepted", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
		})

		Context("when an invalid desire task message is received", func() {
			BeforeEach(func() {
				reader := bytes.NewBufferString("not valid json")
				request.Body = ioutil.NopCloser(reader)
			})

			It("responds with a 400 Bad Request", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("does not send a request to bbs", func() {
				Expect(fakeBBSClient.DesireTaskCallCount()).To(Equal(0))
			})

			It("does not build a task", func() {
				Expect(buildpackBuilder.BuildTaskCallCount()).To(Equal(0))
			})
		})

		Context("when there is an error building the task definition", func() {
			BeforeEach(func() {
				buildpackBuilder.BuildTaskReturns(nil, errors.New("boom!"))
			})

			It("returns a StatusBadRequest", func() {
				Expect(buildpackBuilder.BuildTaskCallCount()).To(Equal(1))
				Expect(buildpackBuilder.BuildTaskArgsForCall(0)).To(Equal(&taskRequest))
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("does not send a request to bbs", func() {
				Expect(fakeBBSClient.DesireTaskCallCount()).To(Equal(0))
			})
		})

		Context("when desiring the task fails", func() {
			Context("because of an unknown error", func() {
				BeforeEach(func() {
					fakeBBSClient.DesireTaskReturns(errors.New("boom!"))
				})

				It("returns a StatusBadRequest", func() {
					Expect(fakeBBSClient.DesireTaskCallCount()).To(Equal(1))
					Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when the requested lifecycle does not have a corresponding builder", func() {
			BeforeEach(func() {
				taskRequest.Lifecycle = "something-else"
			})

			It("responds with a 400 Bad Request", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("does not send a request to bbs", func() {
				Expect(fakeBBSClient.DesireTaskCallCount()).To(Equal(0))
			})

			It("does not build a task", func() {
				Expect(buildpackBuilder.BuildTaskCallCount()).To(Equal(0))
			})
		})
	})
})
