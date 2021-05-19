package eirini_controller_test

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		taskName    string
		taskGUID    string
		task        *eiriniv1.Task
		serviceName string
	)

	BeforeEach(func() {
		taskName = "the-task"
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
	})

	JustBeforeEach(func() {
		_, err := fixture.EiriniClientset.
			EiriniV1().
			Tasks(fixture.Namespace).
			Create(context.Background(), task, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		serviceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, taskGUID, 8080, "/")
	})

	Describe("task creation", func() {
		It("runs the task", func() {
			Expect(tests.RequestServiceFn(fixture.Namespace, serviceName, 8080, "/")()).To(ContainSubstring("not Dora"))
		})
	})

	Describe("task deletion", func() {
		JustBeforeEach(func() {
			err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Delete(context.Background(), taskName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("stops the task", func() {
			Eventually(func() error {
				_, err := tests.RequestServiceFn(fixture.Namespace, serviceName, 8080, "/")()

				return err
			}).Should(MatchError(ContainSubstring("context deadline exceeded")))
		})
	})
})
