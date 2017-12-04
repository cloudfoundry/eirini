package bulk_test

import (
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TaskClient", func() {
	var (
		taskClient *bulk.CCTaskClient
		fakeCC     *ghttp.Server
		logger     *lagertest.TestLogger
		httpClient *http.Client
		taskGuid   string
		taskState  *cc_messages.CCTaskState
	)

	BeforeEach(func() {
		fakeCC = ghttp.NewServer()
		logger = lagertest.NewTestLogger("test")
		httpClient = &http.Client{Timeout: time.Second}

		taskClient = &bulk.CCTaskClient{}
		taskGuid = "task-guid-6000"
		taskState = &cc_messages.CCTaskState{
			TaskGuid:              taskGuid,
			CompletionCallbackUrl: fmt.Sprintf("http://utako:luan@%s/internal/v3/tasks/%s/completed", fakeCC.Addr(), taskGuid),
		}
	})

	Describe("FailTask", func() {
		Context("CC responds successfully", func() {
			BeforeEach(func() {
				fakeCC.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/internal/v3/tasks/%s/completed", taskGuid)),
						ghttp.VerifyBasicAuth("utako", "luan"),
						ghttp.VerifyJSON(`{
						"task_guid": "`+taskGuid+`",
						"failed": true,
						"failure_reason": "Unable to determine completion status"
					}`),
						ghttp.RespondWith(200, "{}"),
					),
				)
			})

			It("sends a fail task request to CC", func() {
				err := taskClient.FailTask(logger, taskState, httpClient)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeCC.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("CC responds with a 404 NotFound", func() {
			BeforeEach(func() {
				fakeCC.AppendHandlers(ghttp.RespondWith(404, "{}"))
			})

			It("returns an error", func() {
				err := taskClient.FailTask(logger, taskState, httpClient)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("CC responds with a 400 BadRequest", func() {
			BeforeEach(func() {
				fakeCC.AppendHandlers(ghttp.RespondWith(400, "{}"))
			})

			It("returns an error", func() {
				err := taskClient.FailTask(logger, taskState, httpClient)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
