package k8s_test

import (
	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Desiretask", func() {

	const (
		Namespace       = "tests"
		Image           = "docker.png"
		CCUploaderIP    = "10.10.10.1"
		CertsSecretName = "secret-certs"
	)

	var (
		task       *opi.Task
		desirer    opi.TaskDesirer
		fakeClient kubernetes.Interface
		err        error
	)

	assertGeneralSpec := func(job *batch.Job) {

		labels := map[string]string{
			"guid":        "env-app-id",
			"source_type": "STG",
		}
		automountServiceAccountToken := false
		Expect(job.Name).To(Equal("the-stage-is-yours"))
		Expect(job.Spec.ActiveDeadlineSeconds).To(Equal(int64ptr(900)))
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(v1.RestartPolicyNever))
		Expect(job.Spec.Template.Labels).To(Equal(labels))
		Expect(job.Labels).To(Equal(labels))
		Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(&automountServiceAccountToken))
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

	BeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
		task = &opi.Task{
			Image: Image,
			Env: map[string]string{
				eirini.EnvDownloadURL:        "example.com/download",
				eirini.EnvDropletUploadURL:   "example.com/upload",
				eirini.EnvAppID:              "env-app-id",
				eirini.EnvStagingGUID:        "the-stage-is-yours",
				eirini.EnvCompletionCallback: "example.com/call/me/maybe",
				eirini.EnvEiriniAddress:      "http://opi.cf.internal",
			},
		}
		desirer = &TaskDesirer{
			Namespace:       Namespace,
			CCUploaderIP:    CCUploaderIP,
			CertsSecretName: CertsSecretName,
			Client:          fakeClient,
		}
	})

	Context("When desiring a task", func() {

		BeforeEach(func() {
			Expect(desirer.Desire(task)).To(Succeed())
		})

		It("should desire the task", func() {
			job, getErr := fakeClient.BatchV1().Jobs(Namespace).Get("the-stage-is-yours", meta_v1.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			assertGeneralSpec(job)

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			assertContainer(containers[0], "opi-task")
		})

		Context("and the job already exists", func() {

			It("should return an error", func() {
				Expect(desirer.Desire(task)).To(MatchError(ContainSubstring("job already exists")))
			})
		})
	})

	Context("When desiring a staging task", func() {

		var stagingTask *opi.StagingTask

		assertHostAliases := func(job *batch.Job) {
			Expect(job.Spec.Template.Spec.HostAliases).To(HaveLen(1))
			hostAlias := job.Spec.Template.Spec.HostAliases[0]

			Expect(hostAlias.IP).To(Equal(CCUploaderIP))
			Expect(hostAlias.Hostnames).To(ContainElement("cc-uploader.service.cf.internal"))
		}

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
			assertHostAliases(job)
			assertVolumes(job)
			assertContainerVolumeMount(job)
		}

		BeforeEach(func() {
			stagingTask = &opi.StagingTask{
				DownloaderImage: Image,
				ExecutorImage:   Image,
				UploaderImage:   Image,
				Task: &opi.Task{
					Env: map[string]string{
						eirini.EnvDownloadURL:        "example.com/download",
						eirini.EnvDropletUploadURL:   "example.com/upload",
						eirini.EnvAppID:              "env-app-id",
						eirini.EnvStagingGUID:        "the-stage-is-yours",
						eirini.EnvCompletionCallback: "example.com/call/me/maybe",
						eirini.EnvEiriniAddress:      "http://opi.cf.internal",
					},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(desirer.DesireStaging(stagingTask)).To(Succeed())
		})

		It("should desire the staging task", func() {
			job, getErr := fakeClient.BatchV1().Jobs(Namespace).Get("the-stage-is-yours", meta_v1.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			assertGeneralSpec(job)

			initContainers := job.Spec.Template.Spec.InitContainers
			Expect(initContainers).To(HaveLen(2))

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))

			assertContainer(initContainers[0], "opi-task-downloader")
			assertContainer(initContainers[1], "opi-task-executor")
			assertContainer(containers[0], "opi-task-uploader")
			assertStagingSpec(job)
		})

		Context("When the staging task already exists", func() {
			It("should return an error", func() {
				Expect(desirer.DesireStaging(stagingTask)).To(MatchError(ContainSubstring("job already exists")))

			})
		})

		Context("when the CC Uploader IP is not provided", func() {
			BeforeEach(func() {
				desirer.(*TaskDesirer).CCUploaderIP = ""
			})

			It("should create a Kubernetes job without any HostAliases", func() {
				job, getErr := fakeClient.BatchV1().Jobs(Namespace).Get("the-stage-is-yours", meta_v1.GetOptions{})
				Expect(getErr).ToNot(HaveOccurred())

				Expect(job.Spec.Template.Spec.HostAliases).To(BeNil())
			})
		})
	})

	Context("When deleting a task", func() {

		Context("that already exists", func() {
			BeforeEach(func() {
				Expect(desirer.Desire(task)).To(Succeed())
			})

			It("should delete the job", func() {
				Expect(desirer.Delete("the-stage-is-yours")).To(Succeed())
				_, err = fakeClient.BatchV1().Jobs(Namespace).Get("env-app-id", meta_v1.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("that does not exist", func() {

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
