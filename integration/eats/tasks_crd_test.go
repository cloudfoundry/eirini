package eats_test

import (
	"context"
	"strings"

	testutil "code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks CRD", func() {

	var (
		task *eiriniv1.Task
	)

	BeforeEach(func() {
		task = &eiriniv1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name: "some-task",
			},
			Spec: eiriniv1.TaskSpec{
				Name:               "the-task",
				GUID:               "the-guid",
				AppGUID:            "the-app-guid",
				AppName:            "wavey",
				SpaceName:          "the-space",
				OrgName:            "the-org",
				CompletionCallback: "http://example.com/complete",
				Env: map[string]string{
					"FOO": "BAR",
				},
				Image: "ubuntu",
				Command: []string{
					"bin/bash",
					"-c",
					"sleep 1",
				},
			},
		}
	})

	JustBeforeEach(func() {
		_, err := fixture.EiriniClientset.
			EiriniV1().
			Tasks(fixture.Namespace).
			Create(context.Background(), task, metav1.CreateOptions{})

		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Creating a Task CRD", func() {
		It("creates a corresponding job in the same namespace", func() {
			Eventually(listJobs).Should(HaveLen(1))

			jobs := listJobs()
			job := jobs[0]
			Expect(job.Name).To(Equal("wavey-the-space-the-task"))
			Expect(job.Labels).To(SatisfyAll(
				HaveKeyWithValue(k8s.LabelGUID, task.Spec.GUID),
				HaveKeyWithValue(k8s.LabelAppGUID, task.Spec.AppGUID),
				HaveKeyWithValue(k8s.LabelSourceType, "TASK"),
				HaveKeyWithValue(k8s.LabelName, task.Spec.Name),
			))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))

			taskContainer := job.Spec.Template.Spec.Containers[0]
			Expect(taskContainer.Image).To(Equal("ubuntu"))
			Expect(taskContainer.Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
			Expect(taskContainer.Command).To(Equal([]string{"bin/bash", "-c", "sleep 1"}))

			Eventually(getJobConditions).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(batchv1.JobComplete),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the task image lives in a private registry", func() {
			BeforeEach(func() {
				task.Spec.Image = "eiriniuser/notdora:latest"
				task.Spec.Command = []string{"/bin/echo", "hello"}
				task.Spec.PrivateRegistry = &eiriniv1.PrivateRegistry{
					Server:   "index.docker.io/v1/",
					Username: "eiriniuser",
					Password: testutil.GetEiriniDockerHubPassword(),
				}
			})

			It("runs and completes the job", func() {
				Eventually(getJobConditions).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})))

			})

			It("creates a ImagePullSecret with the credentials", func() {
				Eventually(listJobs).Should(HaveLen(1))

				registrySecretName := getRegistrySecretName()
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

	Describe("Deleting the Task CRD", func() {

		JustBeforeEach(func() {
			Eventually(listJobs).Should(HaveLen(1))

			err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Delete(context.Background(), task.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the corresponding job", func() {
			Eventually(listJobs).Should(BeEmpty())
		})
	})
})

func getRegistrySecretName() string {
	jobs := listJobs()
	imagePullSecrets := jobs[0].Spec.Template.Spec.ImagePullSecrets
	var registrySecretName string
	for _, imagePullSecret := range imagePullSecrets {
		if strings.HasPrefix(imagePullSecret.Name, "wavey-the-space-registry-secret") {
			registrySecretName = imagePullSecret.Name
		}
	}
	Expect(registrySecretName).NotTo(BeEmpty())
	return registrySecretName
}

func getJobConditions() []batchv1.JobCondition {
	jobs := listJobs()
	return jobs[0].Status.Conditions
}

func listJobs() []batchv1.Job {
	jobs, err := fixture.Clientset.
		BatchV1().
		Jobs(fixture.Namespace).
		List(context.Background(), metav1.ListOptions{})

	Expect(err).NotTo(HaveOccurred())
	return jobs.Items
}
