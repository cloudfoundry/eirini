package jobs_test

import (
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/jobs/jobsfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Delete", func() {
	const (
		Image           = "docker.png"
		CertsSecretName = "secret-certs"
		taskGUID        = "task-123"
	)

	var (
		jobGetter          *jobsfakes.FakeJobGetter
		jobDeleter         *jobsfakes.FakeJobDeleter
		job                batchv1.Job
		deleteErr          error
		completionCallback string

		deleter jobs.Deleter
	)

	BeforeEach(func() {
		jobGetter = new(jobsfakes.FakeJobGetter)
		jobDeleter = new(jobsfakes.FakeJobDeleter)

		deleter = jobs.NewDeleter(
			lagertest.NewTestLogger("deletetask"),
			jobGetter,
			jobDeleter,
		)

		job = batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-job",
				Namespace: "my-namespace",
				Annotations: map[string]string{
					jobs.AnnotationCompletionCallback: "the/completion/callback",
					jobs.AnnotationAppName:            "my-app",
					jobs.AnnotationSpaceName:          "my-space",
				},
				Labels: map[string]string{
					jobs.LabelGUID: taskGUID,
				},
			},
		}

		jobGetter.GetByGUIDReturns([]batchv1.Job{job}, nil)
	})

	JustBeforeEach(func() {
		completionCallback, deleteErr = deleter.Delete(ctx, taskGUID)
	})

	It("succeeds", func() {
		Expect(deleteErr).NotTo(HaveOccurred())
	})

	It("deletes the job", func() {
		Expect(jobDeleter.DeleteCallCount()).To(Equal(1))
		_, actualJobNs, actualJobName := jobDeleter.DeleteArgsForCall(0)
		Expect(actualJobNs).To(Equal(job.Namespace))
		Expect(actualJobName).To(Equal(job.Name))
	})

	It("returns the completion callback", func() {
		Expect(completionCallback).To(Equal("the/completion/callback"))
	})

	It("selects the job using the task label guid and the eirini label", func() {
		Expect(jobGetter.GetByGUIDCallCount()).To(Equal(1))
		_, guid, includeCompleted := jobGetter.GetByGUIDArgsForCall(0)
		Expect(guid).To(Equal(taskGUID))
		Expect(includeCompleted).To(Equal(true))
	})

	When("the job does not exist", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns([]batchv1.Job{}, nil)
		})

		It("returns the error", func() {
			Expect(deleteErr).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 0)))
		})

		It("does not call the deleter", func() {
			Expect(jobDeleter.DeleteCallCount()).To(BeZero())
		})
	})

	When("there are multiple jobs with the same guid", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns([]batchv1.Job{{}, {}}, nil)
		})

		It("returns the error", func() {
			Expect(deleteErr).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 2)))
		})

		It("does not call the deleter", func() {
			Expect(jobDeleter.DeleteCallCount()).To(BeZero())
		})
	})

	When("getting the jobs by GUID fails", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns(nil, errors.New("failed to list jobs"))
		})

		It("should return an error", func() {
			Expect(deleteErr).To(MatchError(ContainSubstring("failed to list jobs")))
		})

		It("does not call the deleter", func() {
			Expect(jobDeleter.DeleteCallCount()).To(BeZero())
		})
	})

	When("job deletion fails", func() {
		BeforeEach(func() {
			jobDeleter.DeleteReturns(errors.New("failed to delete"))
		})

		It("returns an error", func() {
			Expect(deleteErr).To(MatchError(ContainSubstring("failed to delete")))
		})
	})
})
