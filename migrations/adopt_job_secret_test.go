package migrations_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Job Registry Secret Adoption", func() {
	var (
		adoptSecretMigration  migrations.AdoptJobRegistrySecret
		job                   runtime.Object
		sharedSecret          *corev1.Secret
		privateRegistrySecret *corev1.Secret
		secretsClient         *migrationsfakes.FakeSecretsClient
		migrateErr            error
	)

	BeforeEach(func() {
		secretsClient = new(migrationsfakes.FakeSecretsClient)
		adoptSecretMigration = migrations.NewAdoptJobRegistrySecret(secretsClient)

		job = &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-job",
				Namespace: "my-namespace",
				Annotations: map[string]string{
					jobs.AnnotationAppName:   "my-app",
					jobs.AnnotationSpaceName: "my-space",
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "shared-secret"},
							{Name: "my-app-my-space-registry-secret-my-guid-foo"},
						},
					},
				},
			},
		}

		sharedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-secret",
				Namespace: "my-namespace",
			},
		}

		privateRegistrySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-my-space-registry-secret-my-guid-foo",
				Namespace: "my-namespace",
			},
		}

		secretsClient.GetStub = func(_ context.Context, namespace, name string) (*corev1.Secret, error) {
			Expect(namespace).To(Equal("my-namespace"))
			switch name {
			case sharedSecret.Name:
				return sharedSecret, nil
			case privateRegistrySecret.Name:
				return privateRegistrySecret, nil
			default:
				return nil, k8serrors.NewNotFound(schema.GroupResource{}, "nope")
			}
		}
	})

	JustBeforeEach(func() {
		migrateErr = adoptSecretMigration.Apply(ctx, job)
	})

	It("succeeds", func() {
		Expect(migrateErr).NotTo(HaveOccurred())
	})

	It("gets the private registry secret", func() {
		Expect(secretsClient.GetCallCount()).To(Equal(1))
		_, actualNamespace, actualName := secretsClient.GetArgsForCall(0)
		Expect(actualNamespace).To(Equal("my-namespace"))
		Expect(actualName).To(Equal("my-app-my-space-registry-secret-my-guid-foo"))
	})

	When("the app name and the space name cannot be used in a secret name", func() {
		BeforeEach(func() {
			job = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-job",
					Namespace: "my-namespace",
					Annotations: map[string]string{
						jobs.AnnotationSpaceName: "втф",
					},
					Labels: map[string]string{
						jobs.LabelGUID: "my-guid",
					},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "shared-secret"},
								{Name: "my-guid-registry-secret-foo"},
							},
						},
					},
				},
			}

			privateRegistrySecret.Name = "my-guid-registry-secret-foo"
		})

		It("gets the private registry secret by task guid", func() {
			Expect(secretsClient.GetCallCount()).To(Equal(1))
			_, actualNamespace, actualName := secretsClient.GetArgsForCall(0)
			Expect(actualNamespace).To(Equal("my-namespace"))
			Expect(actualName).To(Equal("my-guid-registry-secret-foo"))
		})
	})

	It("sets the ownership of the registry credentials secret", func() {
		Expect(secretsClient.SetOwnerCallCount()).To(Equal(1))
		_, actualSecret, actualJob := secretsClient.SetOwnerArgsForCall(0)
		Expect(actualSecret).To(Equal(privateRegistrySecret))
		Expect(actualJob).To(Equal(job))
	})

	When("getting the private registry secret fails", func() {
		BeforeEach(func() {
			secretsClient.GetReturns(nil, errors.New("get-secret-failed"))
		})

		It("returns the error", func() {
			Expect(migrateErr).To(MatchError(ContainSubstring("get-secret-failed")))
		})
	})

	When("setting the owner of the secret fails", func() {
		BeforeEach(func() {
			secretsClient.SetOwnerReturns(nil, errors.New("set-owner-failed"))
		})

		It("returns the error", func() {
			Expect(migrateErr).To(MatchError(ContainSubstring("set-owner-failed")))
		})
	})

	When("the job does not reference a private registry secret", func() {
		BeforeEach(func() {
			job = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-stateful-set",
					Namespace: "my-namespace",
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "shared-secret"},
							},
						},
					},
				},
			}
		})

		It("is noop", func() {
			Expect(secretsClient.GetCallCount()).To(BeZero())
			Expect(secretsClient.SetOwnerCallCount()).To(BeZero())
		})
	})

	When("the migrated object is not a job", func() {
		BeforeEach(func() {
			job = &appsv1.StatefulSet{}
		})

		It("is noop", func() {
			Expect(secretsClient.GetCallCount()).To(BeZero())
			Expect(secretsClient.SetOwnerCallCount()).To(BeZero())
			Expect(migrateErr).To(MatchError(ContainSubstring("expected *batchv1.Job, got: *v1.StatefulSet")))
		})
	})
})
