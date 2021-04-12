package jobs_test

import (
	"fmt"
	"strconv"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/shared"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TaskToJob", func() {
	const (
		image           = "docker.png"
		taskGUID        = "task-123"
		serviceAccount  = "service-account"
		registrySecret  = "registry-secret"
		latestMigration = 1234
	)

	var (
		job                               *batch.Job
		privateRegistrySecret             *corev1.Secret
		task                              *api.Task
		allowAutomountServiceAccountToken bool
	)

	assertGeneralSpec := func(job *batch.Job) {
		automountServiceAccountToken := false
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
		Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(&automountServiceAccountToken))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(Equal(true)))
	}

	assertContainer := func(container corev1.Container, name string) {
		Expect(container.Name).To(Equal(name))
		Expect(container.Image).To(Equal(image))
		Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))

		Expect(container.Env).To(ContainElements(
			corev1.EnvVar{Name: eirini.EnvDownloadURL, Value: "example.com/download"},
			corev1.EnvVar{Name: eirini.EnvDropletUploadURL, Value: "example.com/upload"},
			corev1.EnvVar{Name: eirini.EnvAppID, Value: "env-app-id"},
			corev1.EnvVar{Name: eirini.EnvCFInstanceGUID, ValueFrom: expectedValFrom("metadata.uid")},
			corev1.EnvVar{Name: eirini.EnvCFInstanceInternalIP, ValueFrom: expectedValFrom("status.podIP")},
			corev1.EnvVar{Name: eirini.EnvCFInstanceIP, ValueFrom: expectedValFrom("status.hostIP")},
			corev1.EnvVar{Name: eirini.EnvPodName, ValueFrom: expectedValFrom("metadata.name")},
			corev1.EnvVar{Name: eirini.EnvCFInstanceAddr, Value: ""},
			corev1.EnvVar{Name: eirini.EnvCFInstancePort, Value: ""},
			corev1.EnvVar{Name: eirini.EnvCFInstancePorts, Value: "[]"},
		))
	}

	BeforeEach(func() {
		allowAutomountServiceAccountToken = false
		privateRegistrySecret = nil

		task = &api.Task{
			Image:              image,
			CompletionCallback: "cloud-countroller.io/task/completed",
			Command:            []string{"/lifecycle/launch"},
			AppName:            "my-app",
			Name:               "task-name",
			AppGUID:            "my-app-guid",
			OrgName:            "my-org",
			SpaceName:          "my-space",
			SpaceGUID:          "space-id",
			OrgGUID:            "org-id",
			GUID:               taskGUID,
			Env: map[string]string{
				eirini.EnvDownloadURL:      "example.com/download",
				eirini.EnvDropletUploadURL: "example.com/upload",
				eirini.EnvAppID:            "env-app-id",
			},
			MemoryMB:  1,
			CPUWeight: 2,
			DiskMB:    3,
		}
	})

	JustBeforeEach(func() {
		job = jobs.NewTaskToJobConverter(serviceAccount, registrySecret, allowAutomountServiceAccountToken, latestMigration).Convert(task, privateRegistrySecret)
	})

	It("returns a job for the task with the correct attributes", func() {
		assertGeneralSpec(job)

		Expect(job.Name).To(Equal("my-app-my-space-task-name"))
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
		Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: registrySecret}))

		containers := job.Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(1))
		assertContainer(containers[0], "opi-task")
		Expect(containers[0].Command).To(ConsistOf("/lifecycle/launch"))

		By("setting the expected annotations on the job", func() {
			Expect(job.Annotations).To(SatisfyAll(
				HaveKeyWithValue(jobs.AnnotationAppName, "my-app"),
				HaveKeyWithValue(jobs.AnnotationAppID, "my-app-guid"),
				HaveKeyWithValue(jobs.AnnotationOrgName, "my-org"),
				HaveKeyWithValue(jobs.AnnotationOrgGUID, "org-id"),
				HaveKeyWithValue(jobs.AnnotationSpaceName, "my-space"),
				HaveKeyWithValue(jobs.AnnotationSpaceGUID, "space-id"),
				HaveKeyWithValue(jobs.AnnotationCompletionCallback, "cloud-countroller.io/task/completed"),
				HaveKeyWithValue(corev1.SeccompPodAnnotationKey, corev1.SeccompProfileRuntimeDefault),
			))
		})

		By("setting the expected labels on the job", func() {
			Expect(job.Labels).To(SatisfyAll(
				HaveKeyWithValue(jobs.LabelAppGUID, "my-app-guid"),
				HaveKeyWithValue(jobs.LabelGUID, "task-123"),
				HaveKeyWithValue(jobs.LabelSourceType, "TASK"),
				HaveKeyWithValue(jobs.LabelName, "task-name"),
			))
		})

		By("setting the expected annotations on the associated pod", func() {
			Expect(job.Spec.Template.Annotations).To(SatisfyAll(
				HaveKeyWithValue(jobs.AnnotationAppName, "my-app"),
				HaveKeyWithValue(jobs.AnnotationAppID, "my-app-guid"),
				HaveKeyWithValue(jobs.AnnotationOrgName, "my-org"),
				HaveKeyWithValue(jobs.AnnotationOrgGUID, "org-id"),
				HaveKeyWithValue(jobs.AnnotationSpaceName, "my-space"),
				HaveKeyWithValue(jobs.AnnotationSpaceGUID, "space-id"),
				HaveKeyWithValue(jobs.AnnotationTaskContainerName, "opi-task"),
				HaveKeyWithValue(jobs.AnnotationGUID, "task-123"),
				HaveKeyWithValue(jobs.AnnotationCompletionCallback, "cloud-countroller.io/task/completed"),
				HaveKeyWithValue(corev1.SeccompPodAnnotationKey, corev1.SeccompProfileRuntimeDefault),
			))
		})

		By("setting the expected labels on the associated pod", func() {
			Expect(job.Spec.Template.Labels).To(SatisfyAll(
				HaveKeyWithValue(jobs.LabelAppGUID, "my-app-guid"),
				HaveKeyWithValue(jobs.LabelGUID, "task-123"),
				HaveKeyWithValue(jobs.LabelSourceType, "TASK"),
			))
		})

		By("setting the latest migration annotation", func() {
			Expect(job.Annotations[shared.AnnotationLatestMigration]).To(Equal(strconv.Itoa(latestMigration)))
		})

		By("creating a secret reference with the registry credentials", func() {
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
				corev1.LocalObjectReference{Name: "registry-secret"},
			))
		})
	})

	When("allowAutomountServiceAccountToken is true", func() {
		BeforeEach(func() {
			allowAutomountServiceAccountToken = true
		})

		It("does not set automountServiceAccountToken on the pod spec", func() {
			Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(BeNil())
		})
	})

	When("the app name and space name are too long", func() {
		BeforeEach(func() {
			task.AppName = "app-with-very-long-name"
			task.SpaceName = "space-with-a-very-very-very-very-very-very-long-name"
		})

		It("should truncate the app and space name", func() {
			Expect(job.Name).To(Equal("app-with-very-long-name-space-with-a-ver-task-name"))
		})
	})

	When("the prefix would be invalid", func() {
		BeforeEach(func() {
			task.AppName = ""
			task.SpaceName = ""
		})

		It("should use the guid as the prefix instead", func() {
			Expect(job.Name).To(Equal(fmt.Sprintf("%s-%s", taskGUID, task.Name)))
		})
	})

	When("the task uses a private registry", func() {
		BeforeEach(func() {
			privateRegistrySecret = &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: "the-private-registry-secret",
				},
			}
		})

		It("creates a secret reference with the private registry credentials", func() {
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
				corev1.LocalObjectReference{Name: "registry-secret"},
				corev1.LocalObjectReference{Name: "the-private-registry-secret"},
			))
		})
	})
})
