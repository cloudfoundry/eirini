package integration_test

import (
	"context"
	"fmt"
	"os"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("LRPClient", func() {
	var (
		taskClient *k8s.TaskClient
		task       *api.Task
		taskGUID   string
	)

	BeforeEach(func() {
		taskGUID = tests.GenerateGUID()
		task = &api.Task{
			GUID:    taskGUID,
			Name:    tests.GenerateGUID(),
			Image:   "eirini/busybox",
			Command: []string{"sh", "-c", "sleep 1"},
			Env: map[string]string{
				"FOO": "BAR",
			},
			AppName:   "app-name",
			AppGUID:   "app-guid",
			OrgName:   "org-name",
			OrgGUID:   "org-guid",
			SpaceName: "s",
			SpaceGUID: "s-guid",
			MemoryMB:  1024,
			DiskMB:    2048,
		}
	})

	JustBeforeEach(func() {
		taskClient = createTaskClient(fixture.Namespace)
	})

	Describe("Desire", func() {
		var desireErr error

		JustBeforeEach(func() {
			desireErr = taskClient.Desire(context.Background(), fixture.Namespace, task)
		})

		It("succeeds", func() {
			Expect(desireErr).NotTo(HaveOccurred())
		})

		It("creates a corresponding job in the same namespace", func() {
			allJobs := integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)()
			job := allJobs[0]
			Expect(job.Name).To(Equal(fmt.Sprintf("app-name-s-%s", task.Name)))
			Expect(job.Labels).To(SatisfyAll(
				HaveKeyWithValue(jobs.LabelGUID, task.GUID),
				HaveKeyWithValue(jobs.LabelAppGUID, task.AppGUID),
				HaveKeyWithValue(jobs.LabelSourceType, "TASK"),
				HaveKeyWithValue(jobs.LabelName, task.Name),
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
				task.Image = "eiriniuser/notdora:latest"
				task.Command = []string{"/bin/echo", "hello"}
				task.PrivateRegistry = &api.PrivateRegistry{
					Username: "eiriniuser",
					Password: tests.GetEiriniDockerHubPassword(),
					Server:   util.DockerHubHost,
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
				registrySecretName := integration.GetRegistrySecretName(fixture.Clientset, fixture.Namespace, taskGUID, jobs.PrivateRegistrySecretGenerateName)

				getSecret := func() (*corev1.Secret, error) {
					return fixture.Clientset.
						CoreV1().
						Secrets(fixture.Namespace).
						Get(context.Background(), registrySecretName, metav1.GetOptions{})
				}

				secret, err := getSecret()

				Expect(err).NotTo(HaveOccurred())

				By("creating the secret", func() {
					Expect(secret.Name).To(ContainSubstring(jobs.PrivateRegistrySecretGenerateName))
					Expect(secret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
					Expect(secret.Data).To(HaveKey(".dockerconfigjson"))
				})

				By("setting the owner reference on the secret", func() {
					allJobs := integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)()
					job := allJobs[0]

					var ownerRefs []metav1.OwnerReference
					Eventually(func() []metav1.OwnerReference {
						s, err := getSecret()
						if err != nil {
							return nil
						}

						ownerRefs = s.OwnerReferences

						return ownerRefs
					}).Should(HaveLen(1))

					Expect(ownerRefs[0].Name).To(Equal(job.Name))
					Expect(ownerRefs[0].UID).To(Equal(job.UID))
				})
			})
		})
	})

	Describe("Get", func() {
		var otherNStaskClient *k8s.TaskClient

		JustBeforeEach(func() {
			otherNStaskClient = createTaskClient(fixture.CreateExtraNamespace())

			Expect(taskClient.Desire(context.Background(), fixture.Namespace, task)).To(Succeed())
		})

		It("gets the task by guid", func() {
			actualTask, err := taskClient.Get(context.Background(), taskGUID)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualTask.GUID).To(Equal(task.GUID))
		})

		It("does not get tasks from other namespaces", func() {
			_, err := otherNStaskClient.Get(context.Background(), taskGUID)
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("List", func() {
		var otherNStaskClient *k8s.TaskClient

		JustBeforeEach(func() {
			otherNStaskClient = createTaskClient(fixture.CreateExtraNamespace())

			Expect(taskClient.Desire(context.Background(), fixture.Namespace, task)).To(Succeed())
		})

		It("List all tasks in the workloadsNamespace", func() {
			actualTasks, err := taskClient.List(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(HaveLen(1))
			Expect(actualTasks[0].GUID).To(Equal(task.GUID))
		})

		It("does not list tasks from other namespaces", func() {
			tasks, err := otherNStaskClient.List(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks).To(BeEmpty())
		})
	})

	Describe("Delete", func() {
		JustBeforeEach(func() {
			Expect(taskClient.Desire(context.Background(), fixture.Namespace, task)).To(Succeed())
		})

		It("deletes the job", func() {
			_, err := taskClient.Delete(context.Background(), taskGUID)
			Expect(err).NotTo(HaveOccurred())
			Eventually(integration.ListJobs(fixture.Clientset, fixture.Namespace, taskGUID)).Should(BeEmpty())
		})
	})
})

func createTaskClient(workloadsNamespace string) *k8s.TaskClient {
	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	taskToJobConverter := jobs.NewTaskToJobConverter(
		tests.GetApplicationServiceAccount(),
		"registry-secret",
		false,
		123,
	)

	return k8s.NewTaskClient(
		logger,
		client.NewJob(fixture.Clientset, workloadsNamespace),
		client.NewSecret(fixture.Clientset),
		taskToJobConverter,
	)
}
