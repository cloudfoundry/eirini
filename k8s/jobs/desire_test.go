package jobs_test

import (
	"encoding/base64"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/jobs/jobsfakes"
	"code.cloudfoundry.org/eirini/k8s/shared/sharedfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desire", func() {
	const (
		image    = "docker.png"
		taskGUID = "task-123"
	)

	var (
		jobCreator         *jobsfakes.FakeJobCreator
		secretCreator      *jobsfakes.FakeSecretCreator
		taskToJobConverter *jobsfakes.FakeTaskToJobConverter
		desireOpt          *sharedfakes.FakeOption

		job       *batch.Job
		task      *opi.Task
		desireErr error

		desirer jobs.Desirer
	)

	BeforeEach(func() {
		job = &batch.Job{}

		desireOpt = new(sharedfakes.FakeOption)
		desireOpt.Stub = func(resource interface{}) error {
			Expect(resource).To(BeAssignableToTypeOf(&batch.Job{}))
			j, ok := resource.(*batch.Job)
			Expect(ok).To(BeTrue())
			Expect(j.Namespace).NotTo(BeEmpty())

			return nil
		}

		jobCreator = new(jobsfakes.FakeJobCreator)
		secretCreator = new(jobsfakes.FakeSecretCreator)
		taskToJobConverter = new(jobsfakes.FakeTaskToJobConverter)
		taskToJobConverter.ConvertReturns(job)

		task = &opi.Task{
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

		desirer = jobs.NewDesirer(
			lagertest.NewTestLogger("desiretask"),
			taskToJobConverter,
			jobCreator,
			secretCreator,
		)
	})

	JustBeforeEach(func() {
		desireErr = desirer.Desire(ctx, "app-namespace", task, desireOpt.Spy)
	})

	It("succeeds", func() {
		Expect(desireErr).NotTo(HaveOccurred())
	})

	It("creates a job", func() {
		Expect(jobCreator.CreateCallCount()).To(Equal(1))
		_, actualNs, actualJob := jobCreator.CreateArgsForCall(0)
		Expect(actualNs).To(Equal("app-namespace"))
		Expect(actualJob).To(Equal(job))
	})

	When("creating the job fails", func() {
		BeforeEach(func() {
			jobCreator.CreateReturns(nil, errors.New("create-failed"))
		})

		It("returns an error", func() {
			Expect(desireErr).To(MatchError(ContainSubstring("create-failed")))
		})
	})

	It("converts the task to job", func() {
		Expect(taskToJobConverter.ConvertCallCount()).To(Equal(1))
		Expect(taskToJobConverter.ConvertArgsForCall(0)).To(Equal(task))
	})

	It("sets the job namespace", func() {
		Expect(job.Namespace).To(Equal("app-namespace"))
	})

	It("applies the desire options after setting the job namespace", func() {
		Expect(desireOpt.CallCount()).To(Equal(1))
		Expect(desireOpt.ArgsForCall(0)).To(Equal(job))
	})

	When("applying an option fails", func() {
		BeforeEach(func() {
			desireOpt.Returns(errors.New("opt-error"))
		})

		It("returns an error", func() {
			Expect(desireErr).To(MatchError(ContainSubstring("opt-error")))
		})
	})

	When("the task uses a private registry", func() {
		BeforeEach(func() {
			task.PrivateRegistry = &opi.PrivateRegistry{
				Server:   "some-server",
				Username: "username",
				Password: "password",
			}
			secretCreator.CreateReturns(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "the-generated-secret-name"}}, nil)
		})

		It("creates a secret with the registry credentials", func() {
			Expect(secretCreator.CreateCallCount()).To(Equal(1))
			_, namespace, actualSecret := secretCreator.CreateArgsForCall(0)
			Expect(namespace).To(Equal("app-namespace"))
			Expect(actualSecret.GenerateName).To(Equal("my-app-my-space-registry-secret-"))
			Expect(actualSecret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
			Expect(actualSecret.StringData).To(
				HaveKeyWithValue(
					".dockerconfigjson",
					fmt.Sprintf(
						`{"auths":{"some-server":{"username":"username","password":"password","auth":"%s"}}}`,
						base64.StdEncoding.EncodeToString([]byte("username:password")),
					),
				),
			)

			Expect(jobCreator.CreateCallCount()).To(Equal(1))
			_, _, job = jobCreator.CreateArgsForCall(0)

			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
				corev1.LocalObjectReference{Name: "the-generated-secret-name"},
			))
		})

		When("creating the secret fails", func() {
			BeforeEach(func() {
				secretCreator.CreateReturns(nil, errors.New("create-secret-err"))
			})

			It("returns an error", func() {
				Expect(desireErr).To(MatchError(ContainSubstring("create-secret-err")))
			})
		})
	})
})
