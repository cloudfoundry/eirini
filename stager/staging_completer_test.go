package stager_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("StagingCompleter", func() {

	var (
		server           *ghttp.Server
		task             *models.TaskCallbackResponse
		handlers         []http.HandlerFunc
		stagingCompleter *stager.CallbackStagingCompleter
		err              error
	)

	const retries = 10

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
		logger := lagertest.NewTestLogger("test")
		stagingCompleter = &stager.CallbackStagingCompleter{
			Logger:     logger,
			HTTPClient: &http.Client{},
			Retries:    retries,
			Delay:      10 * time.Millisecond,
		}
		err = stagingCompleter.CompleteStaging(task)
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
				ghttp.RespondWith(http.StatusInternalServerError, nil),
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should retry configured amount of times", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(retries))
		})
	})
})
