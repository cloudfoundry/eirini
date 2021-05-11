package integration_test

import (
	"time"

	"code.cloudfoundry.org/eirini/k8s/runtimeclient"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		task       *eiriniv1.Task
		taskClient *runtimeclient.Tasks
	)

	BeforeEach(func() {
		task = createTask(fixture.Namespace, "a-task")
		taskClient = runtimeclient.NewTasks(fixture.RuntimeClient)
	})

	Describe("GetTask", func() {
		It("gets a Task by namespace and name", func() {
			actualTask, err := taskClient.GetTask(ctx, fixture.Namespace, task.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTask).To(Equal(task))
		})

		When("the task doesn't exist", func() {
			It("fails", func() {
				_, err := taskClient.GetTask(ctx, fixture.Namespace, "a-non-existing-task")
				Expect(err).To(MatchError(ContainSubstring(`"a-non-existing-task" not found`)))
			})
		})
	})

	Describe("UpdateTaskStatus", func() {
		It("updates the task status, merging it with the existing one", func() {
			now := time.Now()
			startTime := metaTime(now)
			endTime := metaTime(now.Add(time.Hour))

			err := taskClient.UpdateTaskStatus(ctx, task, eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskRunning,
				StartTime:       startTime,
			})
			Expect(err).NotTo(HaveOccurred())

			updatedTask, err := taskClient.GetTask(ctx, fixture.Namespace, task.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedTask.Status.ExecutionStatus).To(Equal(eiriniv1.TaskRunning))
			Expect(updatedTask.Status.StartTime).To(Equal(startTime))

			err = taskClient.UpdateTaskStatus(ctx, task, eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskFailed,
				EndTime:         endTime,
			})
			Expect(err).NotTo(HaveOccurred())

			updatedTask, err = taskClient.GetTask(ctx, fixture.Namespace, task.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedTask.Status.ExecutionStatus).To(Equal(eiriniv1.TaskFailed))
			Expect(updatedTask.Status.StartTime).To(Equal(startTime))
			Expect(updatedTask.Status.EndTime).To(Equal(endTime))
		})

		When("an error occurs", func() {
			It("fails", func() {
				// ???
			})
		})
	})

	Describe("GetJobForTask", func() {
	})
})

func metaTime(t time.Time) *metav1.Time {
	now := metav1.NewTime(t)

	return &now
}
