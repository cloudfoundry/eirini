package k8s_test

import (
	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Desiretask", func() {

	const (
		Namespace       = "tests"
		Image           = "docker.png"
		CertsSecretName = "secret-certs"
		stagingGUID     = "staging-123"
	)

	var (
		task          *opi.Task
		desirer       opi.TaskDesirer
		fakeJobClient *k8sfakes.FakeJobClient
		job           *batch.Job
	)

	assertGeneralSpec := func(job *batch.Job) {
		automountServiceAccountToken := false
		Expect(job.Name).To(Equal("the-stage-is-yours"))
		Expect(job.Spec.ActiveDeadlineSeconds).To(Equal(int64ptr(900)))
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(v1.RestartPolicyNever))
		Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(&automountServiceAccountToken))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(Equal(true)))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsUser).To(PointTo(Equal(int64(2000))))
	}

	assertContainer := func(container v1.Container, name string) {
		Expect(container.Name).To(Equal(name))
		Expect(container.Image).To(Equal(Image))
		Expect(container.ImagePullPolicy).To(Equal(v1.PullAlways))

		expectedValFrom := func(fieldPath string) *v1.EnvVarSource {
			return &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					APIVersion: "",
					FieldPath:  fieldPath,
				},
			}
		}

		Expect(container.Env).To(ConsistOf(
			v1.EnvVar{Name: eirini.EnvDownloadURL, Value: "example.com/download"},
			v1.EnvVar{Name: eirini.EnvDropletUploadURL, Value: "example.com/upload"},
			v1.EnvVar{Name: eirini.EnvAppID, Value: "env-app-id"},
			v1.EnvVar{Name: eirini.EnvStagingGUID, Value: "the-stage-is-yours"},
			v1.EnvVar{Name: eirini.EnvCompletionCallback, Value: "example.com/call/me/maybe"},
			v1.EnvVar{Name: eirini.EnvEiriniAddress, Value: "http://opi.cf.internal"},
			v1.EnvVar{Name: eirini.EnvCFInstanceInternalIP, ValueFrom: expectedValFrom("status.podIP")},
			v1.EnvVar{Name: eirini.EnvCFInstanceIP, ValueFrom: expectedValFrom("status.podIP")},
			v1.EnvVar{Name: eirini.EnvPodName, ValueFrom: expectedValFrom("metadata.name")},
			v1.EnvVar{Name: eirini.EnvCFInstanceAddr, Value: ""},
			v1.EnvVar{Name: eirini.EnvCFInstancePort, Value: ""},
			v1.EnvVar{Name: eirini.EnvCFInstancePorts, Value: "[]"},
		))
	}

	assertExecutorContainer := func(container v1.Container, cpu uint8, mem, disk int64) {
		assertContainer(container, "opi-task-executor")
		Expect(container.Resources.Requests.Memory()).To(Equal(resource.NewScaledQuantity(mem, resource.Mega)))
		Expect(container.Resources.Requests.Cpu()).To(Equal(resource.NewScaledQuantity(int64(cpu*10), resource.Milli)))
		Expect(container.Resources.Requests.StorageEphemeral()).To(Equal(resource.NewScaledQuantity(disk, resource.Mega)))
	}

	BeforeEach(func() {
		fakeJobClient = new(k8sfakes.FakeJobClient)
		task = &opi.Task{
			Image:     Image,
			AppName:   "my-app",
			AppGUID:   "my-app-guid",
			OrgName:   "my-org",
			SpaceName: "my-space",
			SpaceGUID: "space-id",
			OrgGUID:   "org-id",
			Env: map[string]string{
				eirini.EnvDownloadURL:        "example.com/download",
				eirini.EnvDropletUploadURL:   "example.com/upload",
				eirini.EnvAppID:              "env-app-id",
				eirini.EnvStagingGUID:        "the-stage-is-yours",
				eirini.EnvCompletionCallback: "example.com/call/me/maybe",
				eirini.EnvEiriniAddress:      "http://opi.cf.internal",
			},
			MemoryMB:  1,
			CPUWeight: 2,
			DiskMB:    3,
		}
		desirer = &TaskDesirer{
			Namespace:       Namespace,
			CertsSecretName: CertsSecretName,
			JobClient:       fakeJobClient,
		}
	})

	Context("When desiring a task", func() {

		BeforeEach(func() {
			Expect(desirer.Desire(task)).To(Succeed())

			Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
			job = fakeJobClient.CreateArgsForCall(0)
			Expect(job.Name).To(Equal("the-stage-is-yours"))
		})

		It("should desire the task", func() {
			assertGeneralSpec(job)

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			assertContainer(containers[0], "opi-task")
		})

		DescribeTable("the task should have the expected annotations", func(key, value string) {
			Expect(job.Annotations).To(HaveKeyWithValue(key, value))
		},
			Entry("AppName", AnnotationAppName, "my-app"),
			Entry("AppGUID", AnnotationAppID, "my-app-guid"),
			Entry("OrgName", AnnotationOrgName, "my-org"),
			Entry("OrgName", AnnotationOrgGUID, "org-id"),
			Entry("SpaceName", AnnotationSpaceName, "my-space"),
			Entry("SpaceGUID", AnnotationSpaceGUID, "space-id"),
		)

		DescribeTable("the task should have the expected labels", func(key, value string) {
			Expect(job.Labels).To(HaveKeyWithValue(key, value))
		},
			Entry("AppGUID", LabelAppGUID, "my-app-guid"),
			Entry("LabelGUID", LabelGUID, "env-app-id"),
		)

		DescribeTable("the pod associated with the task should have the expected annotations", func(key, value string) {
			Expect(job.Spec.Template.Annotations).To(HaveKeyWithValue(key, value))
		},
			Entry("AppName", AnnotationAppName, "my-app"),
			Entry("AppGUID", AnnotationAppID, "my-app-guid"),
			Entry("OrgName", AnnotationOrgName, "my-org"),
			Entry("OrgName", AnnotationOrgGUID, "org-id"),
			Entry("SpaceName", AnnotationSpaceName, "my-space"),
			Entry("SpaceGUID", AnnotationSpaceGUID, "space-id"),
		)

		DescribeTable("the pod associated with the task should have the expected labels", func(key, value string) {
			Expect(job.Spec.Template.Labels).To(HaveKeyWithValue(key, value))
		},
			Entry("AppGUID", LabelAppGUID, "my-app-guid"),
			Entry("LabelGUID", LabelGUID, "env-app-id"),
		)

		It("should not have staging specific labels", func() {
			Expect(job.Labels[LabelSourceType]).NotTo(Equal("STG"))
			Expect(job.Labels).NotTo(HaveKey(LabelStagingGUID))
		})

		Context("and the job already exists", func() {
			BeforeEach(func() {
				fakeJobClient.CreateReturns(nil, errors.New("job already exists"))
			})

			It("should return an error", func() {
				Expect(desirer.Desire(task)).To(MatchError(ContainSubstring("job already exists")))
			})
		})
	})

	Context("When desiring a staging task", func() {

		var (
			stagingTask *opi.StagingTask
		)

		assertVolumes := func(job *batch.Job) {
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(4))
			Expect(job.Spec.Template.Spec.Volumes).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal(eirini.CertsVolumeName),
					"VolumeSource": Equal(v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{SecretName: "secret-certs"},
					}),
				}),
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal(eirini.RecipeOutputName),
				}),
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal(eirini.RecipeBuildPacksName),
				}),
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal(eirini.RecipeWorkspaceName),
				}),
			))
		}

		assertContainerVolumeMount := func(job *batch.Job) {
			buildpackVolumeMatcher := MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(eirini.RecipeBuildPacksName),
				"ReadOnly":  Equal(false),
				"MountPath": Equal(eirini.RecipeBuildPacksDir),
			})
			certsVolumeMatcher := MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(eirini.CertsVolumeName),
				"ReadOnly":  Equal(true),
				"MountPath": Equal(eirini.CertsMountPath),
			})
			workspaceVolumeMatcher := MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(eirini.RecipeWorkspaceName),
				"ReadOnly":  Equal(false),
				"MountPath": Equal(eirini.RecipeWorkspaceDir),
			})
			outputVolumeMatcher := MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(eirini.RecipeOutputName),
				"ReadOnly":  Equal(false),
				"MountPath": Equal(eirini.RecipeOutputLocation),
			})

			downloaderVolumeMounts := job.Spec.Template.Spec.InitContainers[0].VolumeMounts
			Expect(downloaderVolumeMounts).To(ConsistOf(
				buildpackVolumeMatcher,
				certsVolumeMatcher,
				workspaceVolumeMatcher,
			))

			executorVolumeMounts := job.Spec.Template.Spec.InitContainers[1].VolumeMounts
			Expect(executorVolumeMounts).To(ConsistOf(
				buildpackVolumeMatcher,
				certsVolumeMatcher,
				workspaceVolumeMatcher,
				outputVolumeMatcher,
			))

			uploaderVolumeMounts := job.Spec.Template.Spec.Containers[0].VolumeMounts
			Expect(uploaderVolumeMounts).To(ConsistOf(
				certsVolumeMatcher,
				outputVolumeMatcher,
			))
		}

		assertStagingSpec := func(job *batch.Job) {
			assertVolumes(job)
			assertContainerVolumeMount(job)
		}

		BeforeEach(func() {
			stagingTask = &opi.StagingTask{
				DownloaderImage: Image,
				ExecutorImage:   Image,
				UploaderImage:   Image,
				StagingGUID:     stagingGUID,
				Task:            task,
			}

			Expect(desirer.DesireStaging(stagingTask)).To(Succeed())
		})

		JustBeforeEach(func() {
			Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
			job = fakeJobClient.CreateArgsForCall(0)
			Expect(job.Name).To(Equal("the-stage-is-yours"))
		})

		It("should desire the staging task", func() {

			assertGeneralSpec(job)

			initContainers := job.Spec.Template.Spec.InitContainers
			Expect(initContainers).To(HaveLen(2))

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))

			assertContainer(initContainers[0], "opi-task-downloader")
			assertExecutorContainer(initContainers[1],
				stagingTask.CPUWeight,
				stagingTask.MemoryMB,
				stagingTask.DiskMB,
			)
			assertContainer(containers[0], "opi-task-uploader")
			assertStagingSpec(job)

		})

		DescribeTable("the task should have the expected labels", func(key, value string) {
			Expect(job.Labels).To(HaveKeyWithValue(key, value))
		},
			Entry("AppGUID", LabelAppGUID, "my-app-guid"),
			Entry("LabelGUID", LabelGUID, "env-app-id"),
			Entry("LabelSourceType", LabelSourceType, "STG"),
			Entry("LabelStagingGUID", LabelStagingGUID, stagingGUID),
		)

		Context("When the staging task already exists", func() {
			BeforeEach(func() {
				fakeJobClient.CreateReturns(nil, errors.New("job already exists"))
			})

			It("should return an error", func() {
				Expect(desirer.DesireStaging(stagingTask)).To(MatchError(ContainSubstring("job already exists")))

			})
		})
	})

	Context("When deleting a task", func() {

		It("should delete the job", func() {
			Expect(desirer.Delete("the-stage-is-yours")).To(Succeed())

			Expect(fakeJobClient.DeleteCallCount()).To(Equal(1))
			jobName, _ := fakeJobClient.DeleteArgsForCall(0)
			Expect(jobName).To(Equal("the-stage-is-yours"))
		})

		Context("that does not exist", func() {
			BeforeEach(func() {
				fakeJobClient.DeleteReturns(errors.New("job does not exist"))
			})

			It("should return an error", func() {
				Expect(desirer.Delete("the-stage-is-yours")).To(MatchError(ContainSubstring("job does not exist")))
			})
		})

	})
})

func int64ptr(i int) *int64 {
	u := int64(i)
	return &u
}
