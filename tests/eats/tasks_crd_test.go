package eats_test

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks CRD [needs-logs-for: eirini-controller]", func() {
	var (
		task            *eiriniv1.Task
		taskName        string
		taskGUID        string
		taskDeleteOpts  metav1.DeleteOptions
		taskServiceName string
		port            int32
		ctx             context.Context
	)

	BeforeEach(func() {
		port = 8080

		taskName = tests.GenerateGUID()
		taskGUID = tests.GenerateGUID()
		task = &eiriniv1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name: taskName,
			},
			Spec: eiriniv1.TaskSpec{
				Name:      taskName,
				GUID:      taskGUID,
				AppGUID:   "the-app-guid",
				AppName:   "wavey",
				SpaceName: "the-space",
				OrgName:   "the-org",
				Env: map[string]string{
					"FOO": "BAR",
				},
				Image:   "eirini/dorini",
				Command: []string{"/notdora"},
			},
		}

		ctx = context.Background()
	})

	getTaskStatus := func() (eiriniv1.TaskStatus, error) {
		runningTask, err := fixture.EiriniClientset.
			EiriniV1().
			Tasks(fixture.Namespace).
			Get(ctx, taskName, metav1.GetOptions{})
		if err != nil {
			return eiriniv1.TaskStatus{}, err
		}

		return runningTask.Status, nil
	}

	Describe("Creating a Task CRD", func() {
		JustBeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Create(ctx, task, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())

			taskServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, taskGUID, port)
		})

		It("runs the task", func() {
			Eventually(tests.RequestServiceFn(fixture.Namespace, taskServiceName, port, "/")).Should(ContainSubstring("Dora!"))
			Eventually(getTaskStatus).Should(MatchFields(IgnoreExtras, Fields{
				"ExecutionStatus": Equal(eiriniv1.TaskRunning),
				"StartTime":       Not(BeZero()),
			}))
		})

		When("the task image lives in a private registry", func() {
			BeforeEach(func() {
				task.Spec.Image = "eiriniuser/notdora:latest"
				task.Spec.PrivateRegistry = &eiriniv1.PrivateRegistry{
					Username: "eiriniuser",
					Password: tests.GetEiriniDockerHubPassword(),
				}
				port = 8888
			})

			It("runs the task", func() {
				Eventually(tests.RequestServiceFn(fixture.Namespace, taskServiceName, port, "/")).Should(ContainSubstring("Dora!"))
			})
		})

		When("the task completes successfully", func() {
			BeforeEach(func() {
				task.Spec.Image = "eirini/busybox"
				task.Spec.Command = []string{"echo", "hello"}
			})

			It("marks the Task as succeeded", func() {
				Eventually(getTaskStatus).Should(MatchFields(IgnoreExtras, Fields{
					"ExecutionStatus": Equal(eiriniv1.TaskSucceeded),
					"StartTime":       Not(BeZero()),
					"EndTime":         Not(BeZero()),
				}))
			})
		})

		When("the task fails", func() {
			BeforeEach(func() {
				task.Spec.Image = "eirini/busybox"
				task.Spec.Command = []string{"false"}
			})

			It("marks the Task as failed", func() {
				Eventually(getTaskStatus).Should(MatchFields(IgnoreExtras, Fields{
					"ExecutionStatus": Equal(eiriniv1.TaskFailed),
					"StartTime":       Not(BeZero()),
					"EndTime":         Not(BeZero()),
				}))
			})
		})
	})

	Describe("Deleting the Task CRD", func() {
		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Create(context.Background(), task, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			taskServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, taskGUID, port)
			Eventually(tests.RequestServiceFn(fixture.Namespace, taskServiceName, port, "/")).Should(ContainSubstring("Dora!"))
		})

		JustBeforeEach(func() {
			err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Delete(context.Background(), taskName, taskDeleteOpts)
			Expect(err).NotTo(HaveOccurred())
		})

		It("kills the task", func() {
			// better to check Task status here, once that is available
			Eventually(func() error {
				_, err := tests.RequestServiceFn(fixture.Namespace, taskServiceName, port, "/")()

				return err
			}, "20s").Should(HaveOccurred())
		})
	})
})
