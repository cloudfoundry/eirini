package task_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Reporter", func() {

	var (
		reporter task.StateReporter
		server   *ghttp.Server
		logger   *lagertest.TestLogger
		handlers []http.HandlerFunc
		job      *batchv1.Job
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("task-reporter-test")

		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("PUT", "/tasks/the-task-guid/completed"),
		}
	})

	JustBeforeEach(func() {
		server = ghttp.NewServer()
		server.AppendHandlers(
			ghttp.CombineHandlers(handlers...),
		)

		reporter = task.StateReporter{
			EiriniAddress: server.URL(),
			Client:        &http.Client{},
			Logger:        logger,
		}

		job = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					k8s.LabelGUID: "the-task-guid",
				},
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type: batchv1.JobComplete,
					},
				},
			},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	It("completes the task execution in eirini", func() {
		reporter.Report(job)
		Expect(server.ReceivedRequests()).To(HaveLen(1))
	})

	When("job has not completed", func() {
		JustBeforeEach(func() {
			job.Status = batchv1.JobStatus{}
		})

		It("doesn't send anything to eirini", func() {
			reporter.Report(job)
			Expect(server.ReceivedRequests()).To(HaveLen(0))
		})
	})

	When("the eirini server returns an unexpected status code", func() {
		BeforeEach(func() {
			handlers = []http.HandlerFunc{
				ghttp.VerifyRequest("PUT", "/tasks/the-task-guid/completed"),
				ghttp.RespondWith(http.StatusBadGateway, "potato"),
			}
		})

		It("logs the error", func() {
			reporter.Report(job)
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("task-reporter-test.cannot send task status response"))
			Expect(log.Data).To(HaveKeyWithValue("error", "request not successful: status=502 potato"))
		})
	})
})
