package integration_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/eirini/k8s/crclient"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		task          *eiriniv1.Task
		tasksCrClient *crclient.Tasks
	)

	BeforeEach(func() {
		task = &eiriniv1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-task",
			},
			Spec: eiriniv1.TaskSpec{
				Command: []string{"echo"},
			},
		}

		var err error
		task, err = fixture.EiriniClientset.EiriniV1().Tasks(fixture.Namespace).Create(context.Background(), task, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		tasksCrClient = crclient.NewTasks(fixture.RuntimeClient)
	})

	Describe("GetTask", func() {
		It("gets a Task by namespace and name", func() {
			actualTask, err := tasksCrClient.GetTask(ctx, fixture.Namespace, task.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTask).To(Equal(task))
		})

		When("the task doesn't exist", func() {
			It("fails", func() {
				_, err := tasksCrClient.GetTask(ctx, fixture.Namespace, "a-non-existing-task")
				Expect(err).To(MatchError(ContainSubstring(`"a-non-existing-task" not found`)))
			})
		})
	})

	Describe("UpdateTaskStatus", func() {
		It("updates the task status, merging it with the existing one", func() {
			now := time.Now()
			startTime := metaTime(now)
			endTime := metaTime(now.Add(time.Hour))

			err := tasksCrClient.UpdateTaskStatus(ctx, task, eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskRunning,
				StartTime:       startTime,
			})
			Expect(err).NotTo(HaveOccurred())

			updatedTask, err := tasksCrClient.GetTask(ctx, fixture.Namespace, task.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedTask.Status.ExecutionStatus).To(Equal(eiriniv1.TaskRunning))
			Expect(updatedTask.Status.StartTime).To(Equal(startTime))

			err = tasksCrClient.UpdateTaskStatus(ctx, task, eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskFailed,
				EndTime:         endTime,
			})
			Expect(err).NotTo(HaveOccurred())

			updatedTask, err = tasksCrClient.GetTask(ctx, fixture.Namespace, task.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedTask.Status.ExecutionStatus).To(Equal(eiriniv1.TaskFailed))
			Expect(updatedTask.Status.StartTime).To(Equal(startTime))
			Expect(updatedTask.Status.EndTime).To(Equal(endTime))
		})
	})
})

func metaTime(t time.Time) *metav1.Time {
	now := metav1.NewTime(t)

	return &now
}
