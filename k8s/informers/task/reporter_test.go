package task_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/k8s/informers/task/taskfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Reporter", func() {

	var (
		reporter    task.StateReporter
		server      *ghttp.Server
		logger      *lagertest.TestLogger
		job         *batchv1.Job
		taskDeleter *taskfakes.FakeDeleter
		handlers    []http.HandlerFunc
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("task-reporter-test")
		taskDeleter = new(taskfakes.FakeDeleter)

		server = ghttp.NewServer()
		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("PUT", "/the-callback-url"),
			ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
				TaskGUID: "the-task-guid",
			}),
		}

		reporter = task.StateReporter{
			Client:      &http.Client{},
			Logger:      logger,
			TaskDeleter: taskDeleter,
		}

		job = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					k8s.LabelGUID: "the-task-guid",
				},
				Annotations: map[string]string{
					k8s.AnnotationCompletionCallback: fmt.Sprintf("%s/the-callback-url", server.URL()),
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

	JustBeforeEach(func() {
		server.AppendHandlers(
			ghttp.CombineHandlers(handlers...),
		)

		reporter.Report(job)
	})

	AfterEach(func() {
		server.Close()
	})

	It("notifies the cloud controller", func() {
		Expect(server.ReceivedRequests()).To(HaveLen(1))
	})

	It("deletes the job on kubernetes", func() {
		Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
		Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
	})

	When("the job failed", func() {
		BeforeEach(func() {
			job.Status.Conditions = []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Reason: "because",
				},
			}
			handlers = []http.HandlerFunc{
				ghttp.VerifyRequest("PUT", "/the-callback-url"),
				ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
					TaskGUID:      "the-task-guid",
					Failed:        true,
					FailureReason: "because",
				}),
			}
		})

		It("notifies the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		It("deletes the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
		})
	})

	When("job has not completed", func() {
		BeforeEach(func() {
			job.Status = batchv1.JobStatus{}
		})

		It("doesn't send anything to the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(0))
		})

		It("doesn't send delete the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("the cloud controller returns an unexpected status code", func() {
		BeforeEach(func() {
			server.Reset()
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/the-callback-url"),
					ghttp.RespondWith(http.StatusBadGateway, "potato"),
				),
			)
		})

		It("logs the error", func() {
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("task-reporter-test.cannot send task status response"))
			Expect(log.Data).To(HaveKeyWithValue("error", "request not successful: status=502 potato"))
		})

		It("still deletes the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
		})
	})
})
