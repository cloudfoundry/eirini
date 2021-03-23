package jobs_test

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/jobs/jobsfakes"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Get", func() {
	const taskGUID = "task-123"
	var (
		job       *batch.Job
		err       error
		jobGetter *jobsfakes.FakeJobGetter
		task      *opi.Task
		getter    jobs.Getter
	)

	BeforeEach(func() {
		jobGetter = new(jobsfakes.FakeJobGetter)
		getter = jobs.NewGetter(jobGetter)

		job = &batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					jobs.LabelGUID: taskGUID,
				},
			},
		}

		jobGetter.GetByGUIDReturns([]batch.Job{*job}, nil)
	})

	JustBeforeEach(func() {
		task, err = getter.Get(ctx, taskGUID)
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("requests incompleted jobs from the jobs client", func() {
		Expect(jobGetter.GetByGUIDCallCount()).To(Equal(1))
		_, actualGUID, actualIncludeCompleted := jobGetter.GetByGUIDArgsForCall(0)
		Expect(actualGUID).To(Equal(task.GUID))
		Expect(actualIncludeCompleted).To(BeFalse())
	})

	It("returns the task with the specified task guid", func() {
		Expect(task.GUID).To(Equal(taskGUID))
	})

	When("getting the task fails", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns(nil, errors.New("get-task-error"))
		})

		It("returns the error", func() {
			Expect(err).To(MatchError(ContainSubstring("get-task-error")))
		})
	})

	When("there are no jobs for that task GUID", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns([]batch.Job{}, nil)
		})

		It("returns not found error", func() {
			Expect(err).To(Equal(eirini.ErrNotFound))
		})
	})

	When("there are multiple jobs for that task GUID", func() {
		BeforeEach(func() {
			anotherJob := &batch.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						jobs.LabelGUID: taskGUID,
					},
				},
			}

			jobGetter.GetByGUIDReturns([]batch.Job{*job, *anotherJob}, nil)
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("multiple")))
		})
	})
})
