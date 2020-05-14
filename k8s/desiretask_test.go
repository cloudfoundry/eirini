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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desiretask", func() {

	const (
		Namespace       = "tests"
		Image           = "docker.png"
		CertsSecretName = "secret-certs"
		taskGUID        = "task-123"
	)

	var (
		task              *opi.Task
		desirer           *TaskDesirer
		fakeJobClient     *k8sfakes.FakeJobClient
		fakeSecretsClient *k8sfakes.FakeSecretsClient
		job               *batch.Job
	)

	assertGeneralSpec := func(job *batch.Job) {
		automountServiceAccountToken := false
		Expect(job.Spec.ActiveDeadlineSeconds).To(Equal(int64ptr(900)))
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(v1.RestartPolicyNever))
		Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(&automountServiceAccountToken))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(Equal(true)))
		Expect(job.Spec.Template.Spec.SecurityContext.RunAsUser).To(PointTo(Equal(int64(2000))))
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal("staging-serivce-account"))
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
			v1.EnvVar{Name: eirini.EnvStagingGUID, Value: taskGUID},
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
		fakeSecretsClient = new(k8sfakes.FakeSecretsClient)
		task = &opi.Task{
			Image:     Image,
			AppName:   "my-app",
			Name:      "task-name",
			AppGUID:   "my-app-guid",
			OrgName:   "my-org",
			SpaceName: "my-space",
			SpaceGUID: "space-id",
			OrgGUID:   "org-id",
			GUID:      taskGUID,
			Env: map[string]string{
				eirini.EnvDownloadURL:        "example.com/download",
				eirini.EnvDropletUploadURL:   "example.com/upload",
				eirini.EnvAppID:              "env-app-id",
				eirini.EnvStagingGUID:        taskGUID,
				eirini.EnvCompletionCallback: "example.com/call/me/maybe",
				eirini.EnvEiriniAddress:      "http://opi.cf.internal",
			},
			MemoryMB:  1,
			CPUWeight: 2,
			DiskMB:    3,
		}

		tlsStagingConfigs := []StagingConfigTLS{
			{
				SecretName: "cc-uploader-certs",
				KeyPaths: []KeyPath{
					{Key: "key-to-cc-uploader-cert", Path: "cc-uploader-cert"},
					{Key: "key-to-cc-uploader-priv-key", Path: "cc-uploader-key"},
				},
			},
			{
				SecretName: "eirini-certs",
				KeyPaths: []KeyPath{
					{Key: "key-to-eirini-cert", Path: "eirini-cert"},
					{Key: "key-to-eirini-priv-key", Path: "eirini-key"},
				},
			},
			{
				SecretName: "global-ca",
				KeyPaths: []KeyPath{
					{Key: "key-to-ca", Path: "ca"},
				},
			},
		}

		desirer = &TaskDesirer{
			Namespace:          Namespace,
			TLSConfig:          tlsStagingConfigs,
			JobClient:          fakeJobClient,
			ServiceAccountName: "staging-serivce-account",
			RegistrySecretName: "registry-secret",
			Logger:             lagertest.NewTestLogger("desiretask"),
			SecretsClient:      fakeSecretsClient,
		}
	})

	Context("When desiring a task", func() {
		var err error

		BeforeEach(func() {
			task.Command = []string{"/lifecycle/launch"}
		})

		JustBeforeEach(func() {
			err = desirer.Desire(task)
		})

		It("should create a job for the task with the correct attributes", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
			job = fakeJobClient.CreateArgsForCall(0)

			Expect(job.GenerateName).To(HavePrefix("my-app-my-space-"))

			assertGeneralSpec(job)

			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(v1.LocalObjectReference{Name: "registry-secret"}))
			containers := job.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			assertContainer(containers[0], "opi-task")
			Expect(containers[0].Command).To(ConsistOf("/lifecycle/launch"))

			By("setting the expected annotations on the job", func() {
				Expect(job.Annotations).To(SatisfyAll(
					HaveKeyWithValue(AnnotationAppName, "my-app"),
					HaveKeyWithValue(AnnotationAppID, "my-app-guid"),
					HaveKeyWithValue(AnnotationOrgName, "my-org"),
					HaveKeyWithValue(AnnotationOrgGUID, "org-id"),
					HaveKeyWithValue(AnnotationSpaceName, "my-space"),
					HaveKeyWithValue(AnnotationSpaceGUID, "space-id"),
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
				))
			})

			By("setting the expected labels on the associated pod", func() {
				Expect(job.Spec.Template.Labels).To(SatisfyAll(
					HaveKeyWithValue(LabelAppGUID, "my-app-guid"),
					HaveKeyWithValue(LabelGUID, "task-123"),
					HaveKeyWithValue(LabelSourceType, "TASK"),
				))
			})

			By("not setting stating-specific labels on the job", func() {
				Expect(job.Labels[LabelSourceType]).NotTo(Equal("STG"))
				Expect(job.Labels).NotTo(HaveKey(LabelStagingGUID))
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
				job = fakeJobClient.CreateArgsForCall(0)

				Expect(job.GenerateName).To(Equal(taskGUID + "-"))
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
				fakeSecretsClient.CreateReturns(&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "the-generated-secret-name"}}, nil)
			})

			It("creates a secret with the registry credentials", func() {
				Expect(fakeSecretsClient.CreateCallCount()).To(Equal(1))
				actualSecret := fakeSecretsClient.CreateArgsForCall(0)
				Expect(actualSecret.GenerateName).To(Equal("my-app-my-space-registry-secret-"))
				Expect(actualSecret.Type).To(Equal(v1.SecretTypeDockerConfigJson))
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
				job = fakeJobClient.CreateArgsForCall(0)

				Expect(job.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
					v1.LocalObjectReference{Name: "registry-secret"},
					v1.LocalObjectReference{Name: "the-generated-secret-name"},
				))
			})

			Context("when creating the secret fails", func() {
				BeforeEach(func() {
					fakeSecretsClient.CreateReturns(nil, errors.New("create-secret-err"))
				})

				It("returns an error", func() {
					Expect(err).To(MatchError("create-secret-err"))
				})
			})
		})
	})

	Context("When desiring a staging task", func() {
		var (
			stagingTask *opi.StagingTask
			err         error
		)

		assertVolumes := func(job *batch.Job) {
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(5))
			Expect(job.Spec.Template.Spec.Volumes).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal(eirini.CertsVolumeName),
					"VolumeSource": Equal(v1.VolumeSource{
						Projected: &v1.ProjectedVolumeSource{
							Sources: []v1.VolumeProjection{
								{
									Secret: &v1.SecretProjection{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "cc-uploader-certs",
										},
										Items: []v1.KeyToPath{
											{
												Key:  "key-to-cc-uploader-cert",
												Path: "cc-uploader-cert",
											},
											{
												Key:  "key-to-cc-uploader-priv-key",
												Path: "cc-uploader-key",
											},
										},
									},
								},
								{
									Secret: &v1.SecretProjection{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "eirini-certs",
										},
										Items: []v1.KeyToPath{
											{
												Key:  "key-to-eirini-cert",
												Path: "eirini-cert",
											},
											{
												Key:  "key-to-eirini-priv-key",
												Path: "eirini-key",
											},
										},
									},
								},
								{
									Secret: &v1.SecretProjection{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "global-ca",
										},
										Items: []v1.KeyToPath{
											{
												Key:  "key-to-ca",
												Path: "ca",
											},
										},
									},
								},
							},
						},
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
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal(eirini.BuildpackCacheName),
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
			buildpackCacheVolumeMatcher := MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(eirini.BuildpackCacheName),
				"ReadOnly":  Equal(false),
				"MountPath": Equal(eirini.BuildpackCacheDir),
			})

			downloaderVolumeMounts := job.Spec.Template.Spec.InitContainers[0].VolumeMounts
			Expect(downloaderVolumeMounts).To(ConsistOf(
				buildpackVolumeMatcher,
				certsVolumeMatcher,
				workspaceVolumeMatcher,
				buildpackCacheVolumeMatcher,
			))

			executorVolumeMounts := job.Spec.Template.Spec.InitContainers[1].VolumeMounts
			Expect(executorVolumeMounts).To(ConsistOf(
				buildpackVolumeMatcher,
				certsVolumeMatcher,
				workspaceVolumeMatcher,
				outputVolumeMatcher,
				buildpackCacheVolumeMatcher,
			))

			uploaderVolumeMounts := job.Spec.Template.Spec.Containers[0].VolumeMounts
			Expect(uploaderVolumeMounts).To(ConsistOf(
				certsVolumeMatcher,
				outputVolumeMatcher,
				buildpackCacheVolumeMatcher,
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
				Task:            task,
			}
		})

		JustBeforeEach(func() {
			err = desirer.DesireStaging(stagingTask)
			Expect(fakeJobClient.CreateCallCount()).To(Equal(1))
			job = fakeJobClient.CreateArgsForCall(0)
		})

		It("should desire the staging task", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(job.GenerateName).To(Equal("my-app-my-space-"))

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

		When("the prefix would be invalid", func() {
			BeforeEach(func() {
				task.AppName = ""
				task.SpaceName = ""
			})

			It("should use the guid as the prefix instead", func() {
				Expect(job.GenerateName).To(Equal(taskGUID + "-"))
			})
		})

		DescribeTable("the task should have the expected labels", func(key, value string) {
			Expect(job.Labels).To(HaveKeyWithValue(key, value))
		},
			Entry("AppGUID", LabelAppGUID, "my-app-guid"),
			Entry("LabelGUID", LabelGUID, "task-123"),
			Entry("LabelSourceType", LabelSourceType, "STG"),
			Entry("LabelStagingGUID", LabelStagingGUID, taskGUID),
		)

		Context("When the staging task already exists", func() {
			BeforeEach(func() {
				fakeJobClient.CreateReturns(nil, errors.New("job already exists"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("job already exists")))
			})
		})
	})

	Context("When deleting a task", func() {
		BeforeEach(func() {
			jobs := &batch.JobList{
				Items: []batch.Job{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-job",
						}},
				},
			}
			fakeJobClient.ListReturns(jobs, nil)
		})

		It("should delete a staging job", func() {
			Expect(desirer.DeleteStaging(taskGUID)).To(Succeed())

			Expect(fakeJobClient.ListCallCount()).To(Equal(1))
			Expect(fakeJobClient.ListArgsForCall(0).LabelSelector).To(Equal(fmt.Sprintf("%s=%s", LabelStagingGUID, taskGUID)))

			Expect(fakeJobClient.DeleteCallCount()).To(Equal(1))
			jobName, _ := fakeJobClient.DeleteArgsForCall(0)
			Expect(jobName).To(Equal("my-job"))
		})

		When("deleting a non-staging job", func() {
			It("selects jobs using the task label guid", func() {
				Expect(desirer.Delete(taskGUID)).To(Succeed())

				Expect(fakeJobClient.ListCallCount()).To(Equal(1))
				Expect(fakeJobClient.ListArgsForCall(0).LabelSelector).To(Equal(fmt.Sprintf("%s=%s", LabelGUID, taskGUID)))
			})
		})

		Context("when the job does not exist", func() {
			BeforeEach(func() {
				fakeJobClient.ListReturns(&batch.JobList{}, nil)
			})

			It("should return an error", func() {
				Expect(desirer.DeleteStaging(taskGUID)).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 0)))
				Expect(fakeJobClient.ListCallCount()).To(Equal(1))
				Expect(fakeJobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when there are multiple jobs with the same guid", func() {
			BeforeEach(func() {
				fakeJobClient.ListReturns(&batch.JobList{
					Items: []batch.Job{{}, {}},
				}, nil)
			})

			It("should return an error", func() {
				Expect(desirer.DeleteStaging(taskGUID)).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 2)))
				Expect(fakeJobClient.ListCallCount()).To(Equal(1))
				Expect(fakeJobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when listing the jobs by label fails", func() {

			BeforeEach(func() {
				fakeJobClient.ListReturns(nil, errors.New("failed to list jobs"))
			})

			It("should return an error", func() {
				Expect(desirer.DeleteStaging(taskGUID)).To(MatchError("failed to list jobs"))
				Expect(fakeJobClient.ListCallCount()).To(Equal(1))
				Expect(fakeJobClient.DeleteCallCount()).To(BeZero())
			})

		})

		Context("when the delete fails", func() {
			BeforeEach(func() {
				fakeJobClient.DeleteReturns(errors.New("failed to delete"))
			})

			It("should return an error", func() {
				Expect(desirer.DeleteStaging(taskGUID)).To(MatchError(ContainSubstring("failed to delete")))
			})
		})

	})
})

func int64ptr(i int) *int64 {
	u := int64(i)
	return &u
}

func int32ptr(i int) *int32 {
	u := int32(i)
	return &u
}
