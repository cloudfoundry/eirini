package k8s_test

import (
	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batch "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
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
		Expect(job.Name).To(Equal("the-stage-is-yours"))
		Expect(job.Spec.ActiveDeadlineSeconds).To(Equal(int64ptr(900)))
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(v1.RestartPolicyNever))
		Expect(job.Spec.Template.Labels).To(Equal(map[string]string{"name": "env-app-id"}))
		Expect(job.Labels).To(Equal(map[string]string{"name": "env-app-id"}))
	}

	assertContainer := func(container v1.Container) {
		Expect(container.Name).To(Equal("opi-task"))
		Expect(container.Image).To(Equal(Image))
		Expect(container.ImagePullPolicy).To(Equal(v1.PullAlways))

		Expect(container.Env).To(ConsistOf(
			v1.EnvVar{Name: eirini.EnvDownloadURL, Value: "example.com/download"},
			v1.EnvVar{Name: eirini.EnvDropletUploadURL, Value: "example.com/upload"},
			v1.EnvVar{Name: eirini.EnvAppID, Value: "env-app-id"},
			v1.EnvVar{Name: eirini.EnvStagingGUID, Value: "the-stage-is-yours"},
			v1.EnvVar{Name: eirini.EnvCompletionCallback, Value: "example.com/call/me/maybe"},
			v1.EnvVar{Name: eirini.EnvEiriniAddress, Value: "http://opi.cf.internal"},
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

		JustBeforeEach(func() {
			err = desirer.Desire(task)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should desire the task", func() {
			job, getErr := fakeClient.BatchV1().Jobs(Namespace).Get("the-stage-is-yours", meta_v1.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			assertGeneralSpec(job)

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))

			assertContainer(containers[0])
		})

		Context("and the job already exists", func() {
			BeforeEach(func() {
				err = desirer.Desire(task)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When desiring a staging task", func() {

		toKeyPath := func(key string) v1.KeyToPath {
			return v1.KeyToPath{
				Key:  key,
				Path: key,
			}
		}

		assertHostAliases := func(job *batch.Job) {
			Expect(job.Spec.Template.Spec.HostAliases).To(HaveLen(1))
			hostAlias := job.Spec.Template.Spec.HostAliases[0]

			Expect(hostAlias.IP).To(Equal(CCUploaderIP))
			Expect(hostAlias.Hostnames).To(ContainElement("cc-uploader.service.cf.internal"))
		}

		assertVolumes := func(job *batch.Job) {
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(1))
			volume := job.Spec.Template.Spec.Volumes[0]

			Expect(volume.Name).To(Equal("cc-certs-volume"))
			Expect(volume.VolumeSource.Secret.SecretName).To(Equal("secret-certs"))
			Expect(volume.VolumeSource.Secret.Items).To(ConsistOf(
				toKeyPath(eirini.CCAPICertName),
				toKeyPath(eirini.CCAPIKeyName),
				toKeyPath(eirini.CCUploaderCertName),
				toKeyPath(eirini.CCUploaderKeyName),
				toKeyPath(eirini.CCInternalCACertName),
			))
		}

		assertContainerVolumeMount := func(job *batch.Job) {
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).To(HaveLen(1))
			mount := job.Spec.Template.Spec.Containers[0].VolumeMounts[0]

			Expect(mount.Name).To(Equal("cc-certs-volume"))
			Expect(mount.ReadOnly).To(Equal(true))
			Expect(mount.MountPath).To(Equal("/etc/config/certs"))
		}

		assertStagingSpec := func(job *batch.Job) {
			assertHostAliases(job)
			assertVolumes(job)
			assertContainerVolumeMount(job)
		}

		JustBeforeEach(func() {
			err = desirer.DesireStaging(task)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should desire the staging task", func() {
			job, getErr := fakeClient.BatchV1().Jobs(Namespace).Get("the-stage-is-yours", meta_v1.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			assertGeneralSpec(job)

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))

			assertContainer(containers[0])
			assertStagingSpec(job)
		})

		Context("When the staging task already exists", func() {
			BeforeEach(func() {
				err = desirer.DesireStaging(task)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When deleting a task", func() {

		JustBeforeEach(func() {
			err = desirer.Delete("the-stage-is-yours")
		})

		Context("that already exists", func() {
			BeforeEach(func() {
				err = desirer.Desire(task)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should delete the job", func() {
				_, err = fakeClient.BatchV1().Jobs(Namespace).Get("env-app-id", meta_v1.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("that does not exist", func() {

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

	})
})

func int64ptr(i int) *int64 {
	u := int64(i)
	return &u
}
