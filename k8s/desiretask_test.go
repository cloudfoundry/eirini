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
		Namespace = "tests"
		Image     = "docker.png"
	)

	var (
		task       *opi.Task
		desirer    opi.TaskDesirer
		fakeClient kubernetes.Interface
		err        error
	)

	BeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
		task = &opi.Task{
			Image: Image,
			Env: map[string]string{
				eirini.EnvDownloadURL:        "example.com/download",
				eirini.EnvUploadURL:          "example.com/upload",
				eirini.EnvAppID:              "env-app-id",
				eirini.EnvStagingGUID:        "the-stage-is-yours",
				eirini.EnvCompletionCallback: "example.com/call/me/maybe",
				eirini.EnvCfUsername:         "admin",
				eirini.EnvCfPassword:         "not1234567",
				eirini.EnvAPIAddress:         "api.bosh-lite.com",
				eirini.EnvEiriniAddress:      "http://opi.cf.internal",
			},
		}
		desirer = &TaskDesirer{
			Namespace: Namespace,
			Client:    fakeClient,
		}
	})

	Context("When desiring a task", func() {

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

			Expect(container.Env).To(ConsistOf(
				v1.EnvVar{Name: eirini.EnvDownloadURL, Value: "example.com/download"},
				v1.EnvVar{Name: eirini.EnvUploadURL, Value: "example.com/upload"},
				v1.EnvVar{Name: eirini.EnvAppID, Value: "env-app-id"},
				v1.EnvVar{Name: eirini.EnvStagingGUID, Value: "the-stage-is-yours"},
				v1.EnvVar{Name: eirini.EnvCompletionCallback, Value: "example.com/call/me/maybe"},
				v1.EnvVar{Name: eirini.EnvCfUsername, Value: "admin"},
				v1.EnvVar{Name: eirini.EnvCfPassword, Value: "not1234567"},
				v1.EnvVar{Name: eirini.EnvAPIAddress, Value: "api.bosh-lite.com"},
				v1.EnvVar{Name: eirini.EnvEiriniAddress, Value: "http://opi.cf.internal"},
			))
		}

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
