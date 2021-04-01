package eats_test

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
				Name:               taskName,
				GUID:               taskGUID,
				AppGUID:            "the-app-guid",
				AppName:            "wavey",
				SpaceName:          "the-space",
				OrgName:            "the-org",
				CompletionCallback: "http://example.com/complete",
				Env: map[string]string{
					"FOO": "BAR",
				},
				Image:   "eirini/dorini",
				Command: []string{"/notdora"},
			},
		}
	})

	AfterEach(func() {
		err := fixture.EiriniClientset.
			EiriniV1().
			Tasks(fixture.Namespace).
			DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{},
				metav1.ListOptions{FieldSelector: "metadata.name=" + taskName},
			)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Creating a Task CRD", func() {
		JustBeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Create(context.Background(), task, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())

			taskServiceName = exposeLRP(fixture.Namespace, taskGUID, port)
		})

		It("runs the task", func() {
			Eventually(pingLRPFn(fixture.Namespace, taskServiceName, port, "/"), "20s").Should(ContainSubstring("Dora!"))
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
				Eventually(pingLRPFn(fixture.Namespace, taskServiceName, port, "/")).Should(ContainSubstring("Dora!"))
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
			taskServiceName = exposeLRP(fixture.Namespace, taskGUID, port)
			Eventually(pingLRPFn(fixture.Namespace, taskServiceName, port, "/"), "20s").Should(ContainSubstring("Dora!"))
		})

		JustBeforeEach(func() {
			err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Delete(context.Background(), taskName, taskDeleteOpts)
			Expect(err).NotTo(HaveOccurred())
		})

		It("kills the job container", func() {
			// better to check Task status here, once that is available
			Eventually(func() error {
				_, err := pingLRPFn(fixture.Namespace, taskServiceName, port, "/")()

				return err
			}, "20s").Should(HaveOccurred())
		})
	})
})
