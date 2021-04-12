package jobs_test

import (
	"encoding/base64"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/jobs/jobsfakes"
	"code.cloudfoundry.org/eirini/k8s/shared/sharedfakes"
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
		secretsClient      *jobsfakes.FakeSecretsClient
		taskToJobConverter *jobsfakes.FakeTaskToJobConverter
		desireOpt          *sharedfakes.FakeOption

		job       *batch.Job
		task      *api.Task
		desireErr error

		desirer jobs.Desirer
	)

	BeforeEach(func() {
		job = &batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: "the-job-name",
				UID:  "the-job-uid",
			},
		}

		desireOpt = new(sharedfakes.FakeOption)
		desireOpt.Stub = func(resource interface{}) error {
			Expect(resource).To(BeAssignableToTypeOf(&batch.Job{}))
			j, ok := resource.(*batch.Job)
			Expect(ok).To(BeTrue())
			Expect(j.Namespace).NotTo(BeEmpty())

			return nil
		}

		jobCreator = new(jobsfakes.FakeJobCreator)
		secretsClient = new(jobsfakes.FakeSecretsClient)
		taskToJobConverter = new(jobsfakes.FakeTaskToJobConverter)
		taskToJobConverter.ConvertReturns(job)
		jobCreator.CreateReturns(job, nil)

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

		desirer = jobs.NewDesirer(
			lagertest.NewTestLogger("desiretask"),
			taskToJobConverter,
			jobCreator,
			secretsClient,
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
		var privateRegistrySecret *corev1.Secret

		BeforeEach(func() {
			task.PrivateRegistry = &api.PrivateRegistry{
				Server:   "some-server",
				Username: "username",
				Password: "password",
			}
			privateRegistrySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "the-generated-secret-name",
					Namespace: "app-namespace",
				},
			}
			secretsClient.CreateReturns(privateRegistrySecret, nil)
		})

		It("creates a secret with the registry credentials", func() {
			Expect(secretsClient.CreateCallCount()).To(Equal(1))
			_, namespace, actualSecret := secretsClient.CreateArgsForCall(0)
			Expect(namespace).To(Equal("app-namespace"))
			Expect(actualSecret.GenerateName).To(Equal("private-registry-"))
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
		})

		It("converts the task using the priave registry secret", func() {
			_, actualSecret := taskToJobConverter.ConvertArgsForCall(0)
			Expect(actualSecret).To(Equal(privateRegistrySecret))
		})

		It("sets the ownership of the secret to the job", func() {
			Expect(secretsClient.SetOwnerCallCount()).To(Equal(1))
			_, actualSecret, actualOwner := secretsClient.SetOwnerArgsForCall(0)
			Expect(actualOwner).To(Equal(job))
			Expect(actualSecret).To(Equal(privateRegistrySecret))
		})

		When("creating the secret fails", func() {
			BeforeEach(func() {
				secretsClient.CreateReturns(nil, errors.New("create-secret-err"))
			})

			It("returns an error", func() {
				Expect(desireErr).To(MatchError(ContainSubstring("create-secret-err")))
			})
		})

		When("creating the job fails", func() {
			BeforeEach(func() {
				jobCreator.CreateReturns(nil, errors.New("create-failed"))
			})

			It("returns an error", func() {
				Expect(desireErr).To(MatchError(ContainSubstring("create-failed")))
			})

			It("deletes the secret", func() {
				Expect(secretsClient.DeleteCallCount()).To(Equal(1))
				_, actualNamespace, actualName := secretsClient.DeleteArgsForCall(0)
				Expect(actualNamespace).To(Equal("app-namespace"))
				Expect(actualName).To(Equal("the-generated-secret-name"))
			})

			When("deleting the secret fails", func() {
				BeforeEach(func() {
					secretsClient.DeleteReturns(errors.New("delete-secret-failed"))
				})

				It("returns a job creation error and a note that the secret is not cleaned up", func() {
					Expect(desireErr).To(MatchError(And(ContainSubstring("create-failed"), ContainSubstring("delete-secret-failed"))))
				})
			})
		})

		When("setting the ownership of the secret fails", func() {
			BeforeEach(func() {
				secretsClient.SetOwnerReturns(nil, errors.New("potato"))
			})

			It("returns an error", func() {
				Expect(desireErr).To(MatchError(ContainSubstring("potato")))
			})
		})
	})
})
