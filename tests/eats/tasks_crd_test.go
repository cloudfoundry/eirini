package eats_test

import (
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini/k8s"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks CRD", func() {
	var (
		task           *eiriniv1.Task
		taskName       string
		taskGUID       string
		taskListOpts   metav1.ListOptions
		taskDeleteOpts metav1.DeleteOptions
	)

	listTaskJobs := func() []batchv1.Job {
		jobs, err := fixture.Clientset.
			BatchV1().
			Jobs(fixture.Namespace).
			List(context.Background(), taskListOpts)

		Expect(err).NotTo(HaveOccurred())

		return jobs.Items
	}

	getRegistrySecretName := func() string {
		jobs := listTaskJobs()
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

	getJobConditions := func() []batchv1.JobCondition {
		jobs := listJobs(taskGUID)

		return jobs[0].Status.Conditions
	}

	BeforeEach(func() {
		taskName = tests.GenerateGUID()
		taskGUID = tests.GenerateGUID()
		taskListOpts = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", k8s.LabelGUID, taskGUID),
		}
		bgDelete := metav1.DeletePropagationBackground
		taskDeleteOpts = metav1.DeleteOptions{
			PropagationPolicy: &bgDelete,
		}
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
				Image: "eirini/busybox",
				Command: []string{
					"sh",
					"-c",
					"sleep 1",
				},
			},
		}
	})

	AfterEach(func() {
		err := fixture.EiriniClientset.
			EiriniV1().
			Tasks(fixture.Namespace).
			DeleteCollection(
				context.Background(),
				taskDeleteOpts,
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
		})

		It("creates a corresponding job in the same namespace", func() {
			Eventually(listTaskJobs).Should(HaveLen(1))

			jobs := listTaskJobs()
			job := jobs[0]
			Expect(job.Name).To(Equal(fmt.Sprintf("wavey-the-space-%s", taskName)))
			Expect(job.Labels).To(SatisfyAll(
				HaveKeyWithValue(k8s.LabelGUID, task.Spec.GUID),
				HaveKeyWithValue(k8s.LabelAppGUID, task.Spec.AppGUID),
				HaveKeyWithValue(k8s.LabelSourceType, "TASK"),
				HaveKeyWithValue(k8s.LabelName, task.Spec.Name),
			))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))

			taskContainer := job.Spec.Template.Spec.Containers[0]
			Expect(taskContainer.Image).To(Equal("eirini/busybox"))
			Expect(taskContainer.Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
			Expect(taskContainer.Command).To(Equal([]string{"sh", "-c", "sleep 1"}))

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
					Password: tests.GetEiriniDockerHubPassword(),
				}
			})

			It("runs and completes the job", func() {
				Eventually(listTaskJobs).Should(HaveLen(1))
				Eventually(getJobConditions).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			It("creates a ImagePullSecret with the credentials", func() {
				Eventually(listTaskJobs).Should(HaveLen(1))

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
		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Create(context.Background(), task, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Eventually(listTaskJobs).Should(HaveLen(1))
		})

		JustBeforeEach(func() {
			err := fixture.EiriniClientset.
				EiriniV1().
				Tasks(fixture.Namespace).
				Delete(context.Background(), taskName, taskDeleteOpts)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the corresponding job", func() {
			Eventually(listTaskJobs).Should(BeEmpty())
		})
	})
})
