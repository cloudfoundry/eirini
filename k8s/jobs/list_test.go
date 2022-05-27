package jobs_test

import (
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/jobs/jobsfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("List", func() {
	const taskGUID = "task-123"
	var (
		job       *batch.Job
		tasks     []*api.Task
		jobLister *jobsfakes.FakeJobLister
		lister    jobs.Lister
		err       error
	)

	BeforeEach(func() {
		jobLister = new(jobsfakes.FakeJobLister)
		lister = jobs.NewLister(jobLister)
		job = &batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					jobs.LabelGUID: taskGUID,
				},
			},
		}

		jobLister.ListReturns([]batch.Job{*job}, nil)
	})

	JustBeforeEach(func() {
		tasks, err = lister.List(ctx)
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("excludes completed tasks", func() {
		Expect(jobLister.ListCallCount()).To(Equal(1))
		_, actualIncludeCompleted := jobLister.ListArgsForCall(0)
		Expect(actualIncludeCompleted).To(BeFalse())
	})

	It("returns all tasks", func() {
		Expect(tasks).NotTo(BeEmpty())

		taskGUIDs := []string{}
		for _, task := range tasks {
			taskGUIDs = append(taskGUIDs, task.GUID)
		}

		Expect(taskGUIDs).To(ContainElement(taskGUID))
	})

	When("listing the task fails", func() {
		BeforeEach(func() {
			jobLister.ListReturns(nil, errors.New("list-tasks-error"))
		})

		It("returns the error", func() {
			Expect(err).To(MatchError(ContainSubstring("list-tasks-error")))
		})
	})
})
