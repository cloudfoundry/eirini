package k8s_test

import (
	"fmt"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TaskDeleter", func() {
	const (
		Image            = "docker.png"
		CertsSecretName  = "secret-certs"
		defaultNamespace = "default-ns"
		taskGUID         = "task-123"
	)

	var (
		task          *opi.Task
		deleter       *TaskDeleter
		jobClient     *k8sfakes.FakeJobDeletingClient
		secretDeleter *k8sfakes.FakeSecretsCreatorDeleter
		job           batchv1.Job
	)

	BeforeEach(func() {
		jobClient = new(k8sfakes.FakeJobDeletingClient)
		secretDeleter = new(k8sfakes.FakeSecretsCreatorDeleter)
		task = &opi.Task{
			Image: Image,
			Name:  "task-name",
		}

		deleter = NewTaskDeleter(
			lagertest.NewTestLogger("deletetask"),
			jobClient,
			secretDeleter,
		)

		job = batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-job",
				Namespace: "my-namespace",
				Annotations: map[string]string{
					AnnotationCompletionCallback: "the/completion/callback",
					AnnotationAppName:            "my-app",
					AnnotationSpaceName:          "my-space",
				},
				Labels: map[string]string{
					LabelGUID: taskGUID,
				},
			},
		}

		jobClient.GetByGUIDReturns([]batchv1.Job{job}, nil)
	})

	Describe("Delete", func() {
		It("deletes the job", func() {
			completionCallback, err := deleter.Delete(taskGUID)

			By("succeeding")
			Expect(err).To(Succeed())

			By("returning the task completion callback")
			Expect(completionCallback).To(Equal("the/completion/callback"))

			By("selecting the job using the task label guid and the eirini label")
			Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
			guid, includeCompleted := jobClient.GetByGUIDArgsForCall(0)
			Expect(guid).To(Equal(taskGUID))
			Expect(includeCompleted).To(Equal(true))
		})

		Context("when the job has an owner", func() {
			BeforeEach(func() {
				job.OwnerReferences = []metav1.OwnerReference{
					{
						Kind:       "Something",
						APIVersion: "example.org",
						Name:       "the-something",
					},
				}
				jobClient.GetByGUIDReturns([]batchv1.Job{job}, nil)
			})

			It("does not delete the job", func() {
				_, err := deleter.Delete(taskGUID)
				Expect(err).NotTo(HaveOccurred())
				Expect(jobClient.DeleteCallCount()).To(Equal(0))
			})
		})

		Context("when the job does not exist", func() {
			BeforeEach(func() {
				jobClient.GetByGUIDReturns([]batchv1.Job{}, nil)
			})

			It("should return an error", func() {
				_, err := deleter.Delete(taskGUID)
				Expect(err).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 0)))
				Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
				Expect(jobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when there are multiple jobs with the same guid", func() {
			BeforeEach(func() {
				jobClient.GetByGUIDReturns([]batchv1.Job{{}, {}}, nil)
			})

			It("should return an error", func() {
				_, err := deleter.Delete(taskGUID)
				Expect(err).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 2)))
				Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
				Expect(jobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when the job references image pull secrets", func() {
			var dockerRegistrySecretName string

			BeforeEach(func() {
				dockerRegistrySecretName = fmt.Sprintf("%s-%s-registry-secret-%s", "my-app", "my-space", taskGUID)

				job.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
					{Name: dockerRegistrySecretName},
					{Name: "another-random-secret"},
				}
				jobClient.GetByGUIDReturns([]batchv1.Job{job}, nil)
			})

			It("deletes the docker registry image pull secret only", func() {
				_, err := deleter.Delete(task.GUID)
				Expect(err).NotTo(HaveOccurred())
				Expect(secretDeleter.DeleteCallCount()).To(Equal(1))
				actualNamespace, actualSecretName := secretDeleter.DeleteArgsForCall(0)
				Expect(actualNamespace).To(Equal("my-namespace"))
				Expect(actualSecretName).To(Equal(dockerRegistrySecretName))
			})

			Context("when deleting the docker registry image pull secret fails", func() {
				BeforeEach(func() {
					secretDeleter.DeleteReturns(errors.New("docker-secret-delete-failure"))
				})

				It("returns the error", func() {
					_, err := deleter.Delete(task.GUID)
					Expect(err).To(MatchError(ContainSubstring("docker-secret-delete-failure")))
				})
			})
		})

		Context("when listing the jobs by label fails", func() {
			BeforeEach(func() {
				jobClient.GetByGUIDReturns(nil, errors.New("failed to list jobs"))
			})

			It("should return an error", func() {
				_, err := deleter.Delete(taskGUID)
				Expect(err).To(MatchError("failed to list jobs"))
				Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
				Expect(jobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when the delete fails", func() {
			BeforeEach(func() {
				jobClient.DeleteReturns(errors.New("failed to delete"))
			})

			It("should return an error", func() {
				_, err := deleter.Delete(taskGUID)
				Expect(err).To(MatchError(ContainSubstring("failed to delete")))
			})
		})
	})

	Describe("DeleteStaging", func() {
		It("daletes the job", func() {
			Expect(deleter.DeleteStaging(taskGUID)).To(Succeed())

			Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
			guid, includeCompleted := jobClient.GetByGUIDArgsForCall(0)
			Expect(guid).To(Equal(taskGUID))
			Expect(includeCompleted).To(Equal(true))

			Expect(jobClient.DeleteCallCount()).To(Equal(1))
			namespace, jobName := jobClient.DeleteArgsForCall(0)
			Expect(jobName).To(Equal("my-job"))
			Expect(namespace).To(Equal("my-namespace"))
		})

		Context("when the job does not exist", func() {
			BeforeEach(func() {
				jobClient.GetByGUIDReturns([]batchv1.Job{}, nil)
			})

			It("should return an error", func() {
				Expect(deleter.DeleteStaging(taskGUID)).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 0)))
				Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
				Expect(jobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when there are multiple jobs with the same guid", func() {
			BeforeEach(func() {
				jobClient.GetByGUIDReturns([]batchv1.Job{{}, {}}, nil)
			})

			It("should return an error", func() {
				Expect(deleter.DeleteStaging(taskGUID)).To(MatchError(fmt.Sprintf("job with guid %s should have 1 instance, but it has: %d", taskGUID, 2)))
				Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
				Expect(jobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when the job references image pull secrets", func() {
			var dockerRegistrySecretName string

			BeforeEach(func() {
				dockerRegistrySecretName = fmt.Sprintf("%s-%s-registry-secret-%s", "my-app", "my-space", taskGUID)

				job.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
					{Name: dockerRegistrySecretName},
					{Name: "another-random-secret"},
				}
				jobClient.GetByGUIDReturns([]batchv1.Job{job}, nil)
			})

			It("deletes the docker registry image pull secret only", func() {
				Expect(deleter.DeleteStaging(task.GUID)).To(Succeed())
				Expect(secretDeleter.DeleteCallCount()).To(Equal(1))
				actualNamespace, actualSecretName := secretDeleter.DeleteArgsForCall(0)
				Expect(actualNamespace).To(Equal("my-namespace"))
				Expect(actualSecretName).To(Equal(dockerRegistrySecretName))
			})

			Context("when deleting the docker registry image pull secret fails", func() {
				BeforeEach(func() {
					secretDeleter.DeleteReturns(errors.New("docker-secret-delete-failure"))
				})

				It("returns the error", func() {
					Expect(deleter.DeleteStaging(task.GUID)).To(MatchError(ContainSubstring("docker-secret-delete-failure")))
				})
			})
		})

		Context("when listing the jobs by label fails", func() {
			BeforeEach(func() {
				jobClient.GetByGUIDReturns(nil, errors.New("failed to list jobs"))
			})

			It("should return an error", func() {
				Expect(deleter.DeleteStaging(taskGUID)).To(MatchError("failed to list jobs"))
				Expect(jobClient.GetByGUIDCallCount()).To(Equal(1))
				Expect(jobClient.DeleteCallCount()).To(BeZero())
			})
		})

		Context("when the delete fails", func() {
			BeforeEach(func() {
				jobClient.DeleteReturns(errors.New("failed to delete"))
			})

			It("should return an error", func() {
				Expect(deleter.DeleteStaging(taskGUID)).To(MatchError(ContainSubstring("failed to delete")))
			})
		})
	})
})
