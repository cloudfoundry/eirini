package eirini_controller_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
			Expect(job.Name).To(Equal(fmt.Sprintf("wavey-the-space-%s", taskName)))
			Expect(job.Labels).To(SatisfyAll(
				HaveKeyWithValue(jobs.LabelGUID, task.Spec.GUID),
				HaveKeyWithValue(jobs.LabelAppGUID, task.Spec.AppGUID),
				HaveKeyWithValue(jobs.LabelSourceType, "TASK"),
				HaveKeyWithValue(jobs.LabelName, task.Spec.Name),
			))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))

			taskContainer := job.Spec.Template.Spec.Containers[0]
			Expect(taskContainer.Image).To(Equal("eirini/busybox"))
			Expect(taskContainer.Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
			Expect(taskContainer.Command).To(Equal([]string{"sh", "-c", "sleep 1"}))

			Eventually(integration.GetTaskJobConditions(fixture.Clientset, fixture.Namespace, taskGUID)).Should(
				ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})),
			)
		})

		When("the task image lives in a private registry", func() {
			BeforeEach(func() {
				task.Spec.Image = "eiriniuser/notdora:latest"
				task.Spec.Command = []string{"/bin/echo", "hello"}
				task.Spec.PrivateRegistry = &eiriniv1.PrivateRegistry{
					Server:   "index.docker.io/v1/",
					Username: "eiriniuser",
					Password: tests.GetEiriniDockerHubPassword(),
				}
			})

			It("runs and completes the job", func() {
				allJobs := integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)()
				job := allJobs[0]
				Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
				taskContainer := job.Spec.Template.Spec.Containers[0]
				Expect(taskContainer.Image).To(Equal("eiriniuser/notdora:latest"))

				Eventually(integration.GetTaskJobConditions(fixture.Clientset, fixture.Namespace, taskGUID)).Should(
					ConsistOf(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(batchv1.JobComplete),
						"Status": Equal(corev1.ConditionTrue),
					})),
				)
			})

			It("creates a ImagePullSecret with the credentials", func() {
				registrySecretName := integration.GetRegistrySecretName(fixture.Clientset, fixture.Namespace, taskGUID, "wavey-the-space-registry-secret")
				secret, err := fixture.Clientset.
					CoreV1().
					Secrets(fixture.Namespace).
					Get(context.Background(), registrySecretName, metav1.GetOptions{})

				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(ContainSubstring("registry-secret"))
				Expect(secret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
				Expect(secret.Data).To(HaveKey(".dockerconfigjson"))
			})
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
