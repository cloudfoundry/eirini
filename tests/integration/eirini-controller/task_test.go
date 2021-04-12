package eirini_controller_test

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		taskName string
		taskGUID string
		task     *eiriniv1.Task
	)

	BeforeEach(func() {
		taskName = "the-task"
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
				Image:   "eirini/busybox",
				Command: []string{"sh", "-c", "sleep 1"},
			},
		}
	})

	JustBeforeEach(func() {
		_, err := fixture.EiriniClientset.
			EiriniV1().
			Tasks(fixture.Namespace).
			Create(context.Background(), task, metav1.CreateOptions{})

		Expect(err).NotTo(HaveOccurred())

		Eventually(integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)).Should(HaveLen(1))
	})

	Describe("task creation", func() {
		It("creates a corresponding job in the same namespace", func() {
			allJobs := integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)()
			job := allJobs[0]
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(allJobs[0].Labels).To(HaveKeyWithValue(jobs.LabelGUID, taskGUID))
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

		It("deletes the job", func() {
			Eventually(integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)).Should(HaveLen(0))
		})
	})
})
