package k8s_test

import (
	"encoding/base64"
	"fmt"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TaskDesirer", func() {
	const (
		Image           = "docker.png"
		CertsSecretName = "secret-certs"
		taskGUID        = "task-123"
	)

	var (
		task               *opi.Task
		desirer            *TaskDesirer
		fakeJobClient      *k8sfakes.FakeJobCreatingClient
		fakeSecretsCreator *k8sfakes.FakeSecretsCreator
		job                *batch.Job
		jobNamespace       string
		desireOpts         []DesireOption
	)

	assertGeneralSpec := func(job *batch.Job) {
		automountServiceAccountToken := false
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
		Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(&automountServiceAccountToken))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(Equal(true)))
	}

	assertContainer := func(container corev1.Container, name string) {
		Expect(container.Name).To(Equal(name))
		Expect(container.Image).To(Equal(Image))
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
		fakeJobClient = new(k8sfakes.FakeJobCreatingClient)
		fakeSecretsCreator = new(k8sfakes.FakeSecretsCreator)
		desireOpts = []DesireOption{}
		task = &opi.Task{
			Image:              Image,
			CompletionCallback: "cloud-countroller.io/task/completed",
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

		desirer = NewTaskDesirer(
			lagertest.NewTestLogger("desiretask"),
			fakeJobClient,
			fakeSecretsCreator,
			"service-account",
			"registry-secret",
			false,
		)
	})

	Describe("Desire", func() {
		var err error

		BeforeEach(func() {
			task.Command = []string{"/lifecycle/launch"}
		})

		JustBeforeEach(func() {
			err = desirer.Desire("app-namespace", task, desireOpts...)
		})

		It("should create a job for the task with the correct attributes", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
			jobNamespace, job = fakeJobClient.CreateArgsForCall(0)

			assertGeneralSpec(job)

			Expect(job.Name).To(Equal("my-app-my-space-task-name"))
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal("service-account"))
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "registry-secret"}))

			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			assertContainer(containers[0], "opi-task")
			Expect(containers[0].Command).To(ConsistOf("/lifecycle/launch"))

			By("setting the job's namespace to the app's namespace", func() {
				Expect(jobNamespace).To(Equal("app-namespace"))
			})

			By("setting the expected annotations on the job", func() {
				Expect(job.Annotations).To(SatisfyAll(
					HaveKeyWithValue(AnnotationAppName, "my-app"),
					HaveKeyWithValue(AnnotationAppID, "my-app-guid"),
					HaveKeyWithValue(AnnotationOrgName, "my-org"),
					HaveKeyWithValue(AnnotationOrgGUID, "org-id"),
					HaveKeyWithValue(AnnotationSpaceName, "my-space"),
					HaveKeyWithValue(AnnotationSpaceGUID, "space-id"),
					HaveKeyWithValue(AnnotationCompletionCallback, "cloud-countroller.io/task/completed"),
					HaveKeyWithValue(corev1.SeccompPodAnnotationKey, corev1.SeccompProfileRuntimeDefault),
				))
			})

			By("setting the expected labels on the job", func() {
				Expect(job.Labels).To(SatisfyAll(
					HaveKeyWithValue(LabelAppGUID, "my-app-guid"),
					HaveKeyWithValue(LabelGUID, "task-123"),
					HaveKeyWithValue(LabelSourceType, "TASK"),
					HaveKeyWithValue(LabelName, "task-name"),
				))
			})

			By("setting the expected annotations on the associated pod", func() {
				Expect(job.Spec.Template.Annotations).To(SatisfyAll(
					HaveKeyWithValue(AnnotationAppName, "my-app"),
					HaveKeyWithValue(AnnotationAppID, "my-app-guid"),
					HaveKeyWithValue(AnnotationOrgName, "my-org"),
					HaveKeyWithValue(AnnotationOrgGUID, "org-id"),
					HaveKeyWithValue(AnnotationSpaceName, "my-space"),
					HaveKeyWithValue(AnnotationSpaceGUID, "space-id"),
					HaveKeyWithValue(AnnotationOpiTaskContainerName, "opi-task"),
					HaveKeyWithValue(AnnotationGUID, "task-123"),
					HaveKeyWithValue(AnnotationCompletionCallback, "cloud-countroller.io/task/completed"),
					HaveKeyWithValue(corev1.SeccompPodAnnotationKey, corev1.SeccompProfileRuntimeDefault),
				))
			})

			By("setting the expected labels on the associated pod", func() {
				Expect(job.Spec.Template.Labels).To(SatisfyAll(
					HaveKeyWithValue(LabelAppGUID, "my-app-guid"),
					HaveKeyWithValue(LabelGUID, "task-123"),
					HaveKeyWithValue(LabelSourceType, "TASK"),
				))
			})
		})

		When("allowAutomountServiceAccountToken is true", func() {
			BeforeEach(func() {
				desirer = NewTaskDesirerWithEiriniInstance(
					lagertest.NewTestLogger("desiretask"),
					fakeJobClient,
					fakeSecretsCreator,
					"service-account",
					"registry-secret",
					true,
				)
			})

			It("does not set automountServiceAccountToken on the pod spec", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
				_, job = fakeJobClient.CreateArgsForCall(0)

				Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(BeNil())
			})
		})

		When("the app name and space name are too long", func() {
			BeforeEach(func() {
				task.AppName = "app-with-very-long-name"
				task.SpaceName = "space-with-a-very-very-very-very-very-very-long-name"
			})

			It("should truncate the app and space name", func() {
				Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
				_, job = fakeJobClient.CreateArgsForCall(0)
				Expect(job.Name).To(Equal("app-with-very-long-name-space-with-a-ver-task-name"))
			})
		})

		When("desire options are passed", func() {
			var desireOpt1, desireOpt2 *k8sfakes.FakeDesireOption

			BeforeEach(func() {
				desireOpt1 = new(k8sfakes.FakeDesireOption)
				desireOpt2 = new(k8sfakes.FakeDesireOption)
				desireOpts = []DesireOption{desireOpt1.Spy, desireOpt2.Spy}
			})

			It("executes them all correctly", func() {
				Expect(desireOpt1.CallCount()).To(Equal(1))
				obj := desireOpt1.ArgsForCall(0)
				Expect(obj).To(BeAssignableToTypeOf(&batch.Job{}))
				Expect(desireOpt2.CallCount()).To(Equal(1))
			})

			It("has namespace set on the job", func() {
				Expect(desireOpt1.CallCount()).To(Equal(1))
				obj := desireOpt1.ArgsForCall(0)
				Expect(obj).To(BeAssignableToTypeOf(&batch.Job{}))
				job = obj.(*batch.Job)
				Expect(job.Namespace).To(Equal("app-namespace"))
			})

			When("one of the options fails", func() {
				BeforeEach(func() {
					desireOpt2.Returns(errors.New("boom"))
				})

				It("returns an error", func() {
					Expect(err).To(MatchError(ContainSubstring("boom")))
				})
			})
		})

		When("the prefix would be invalid", func() {
			BeforeEach(func() {
				task.AppName = ""
				task.SpaceName = ""
			})

			It("should use the guid as the prefix instead", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
				_, job = fakeJobClient.CreateArgsForCall(0)

				Expect(job.Name).To(Equal(fmt.Sprintf("%s-%s", taskGUID, task.Name)))
			})
		})

		Context("and the job already exists", func() {
			BeforeEach(func() {
				fakeJobClient.CreateReturns(nil, errors.New("job already exists"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("job already exists")))
			})
		})

		Context("when the job uses a private registry", func() {
			BeforeEach(func() {
				task.PrivateRegistry = &opi.PrivateRegistry{
					Server:   "some-server",
					Username: "username",
					Password: "password",
				}
				fakeSecretsCreator.CreateReturns(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "the-generated-secret-name"}}, nil)
			})

			It("creates a secret with the registry credentials", func() {
				Expect(fakeSecretsCreator.CreateCallCount()).To(Equal(1))
				namespace, actualSecret := fakeSecretsCreator.CreateArgsForCall(0)
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

				Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
				_, job = fakeJobClient.CreateArgsForCall(0)

				Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
					corev1.LocalObjectReference{Name: "registry-secret"},
					corev1.LocalObjectReference{Name: "the-generated-secret-name"},
				))
			})

			Context("when creating the secret fails", func() {
				BeforeEach(func() {
					fakeSecretsCreator.CreateReturns(nil, errors.New("create-secret-err"))
				})

				It("returns an error", func() {
					Expect(err).To(MatchError(ContainSubstring("create-secret-err")))
				})
			})
		})
	})

	Describe("Get", func() {
		var err error

		BeforeEach(func() {
			job = &batch.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelGUID: taskGUID,
					},
				},
			}

			fakeJobClient.GetByGUIDReturns([]batch.Job{*job}, nil)
		})

		JustBeforeEach(func() {
			task, err = desirer.Get(task.GUID)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("requests uncompleted jobs from the jobs client", func() {
			Expect(fakeJobClient.GetByGUIDCallCount()).To(Equal(1))
			actualGUID, actualIncludeCompleted := fakeJobClient.GetByGUIDArgsForCall(0)
			Expect(actualGUID).To(Equal(task.GUID))
			Expect(actualIncludeCompleted).To(BeFalse())
		})

		It("returns the task with the specified task guid", func() {
			Expect(task.GUID).To(Equal(taskGUID))
		})

		When("getting the task fails", func() {
			BeforeEach(func() {
				fakeJobClient.GetByGUIDReturns(nil, errors.New("get-task-error"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("get-task-error")))
			})
		})

		When("there are no jobs for that task GUID", func() {
			BeforeEach(func() {
				fakeJobClient.GetByGUIDReturns([]batch.Job{}, nil)
			})

			It("returns not found error", func() {
				Expect(err).To(Equal(eirini.ErrNotFound))
			})
		})

		When("there are multiple jobs for that task GUID", func() {
			BeforeEach(func() {
				anotherJob := &batch.Job{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							LabelGUID: taskGUID,
						},
					},
				}

				fakeJobClient.GetByGUIDReturns([]batch.Job{*job, *anotherJob}, nil)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("multiple")))
			})
		})
	})

	Describe("List", func() {
		var (
			tasks []*opi.Task
			err   error
		)

		BeforeEach(func() {
			job = &batch.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelGUID: taskGUID,
					},
				},
			}

			fakeJobClient.ListReturns([]batch.Job{*job}, nil)
		})

		JustBeforeEach(func() {
			tasks, err = desirer.List()
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("excludes completed tasks", func() {
			Expect(fakeJobClient.ListCallCount()).To(Equal(1))
			Expect(fakeJobClient.ListArgsForCall(0)).To(BeFalse())
		})

		It("returns all tasks", func() {
			Expect(tasks).NotTo(BeEmpty())

			taskGUIDs := []string{}
			for _, task := range tasks {
				taskGUIDs = append(taskGUIDs, task.GUID)
			}

			Expect(taskGUIDs).To(ContainElement(taskGUID))
		})

		When("listing the task fails", func() {
			BeforeEach(func() {
				fakeJobClient.ListReturns(nil, errors.New("list-tasks-error"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("list-tasks-error")))
			})
		})
	})
})

func int32ptr(i int) *int32 {
	u := int32(i)

	return &u
}
