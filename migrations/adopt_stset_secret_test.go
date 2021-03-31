package migrations_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Statefulset Registry Secret Adoption", func() {
	var (
		adoptSecretMigration  migrations.AdoptStatefulsetRegistrySecret
		stSet                 runtime.Object
		sharedSecret          *corev1.Secret
		privateRegistrySecret *corev1.Secret
		secretsClient         *migrationsfakes.FakeSecretsClient
		migrateErr            error
	)

	BeforeEach(func() {
		secretsClient = new(migrationsfakes.FakeSecretsClient)
		adoptSecretMigration = migrations.NewAdoptStatefulsetRegistrySecret(secretsClient)

		stSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-stateful-set",
				Namespace: "my-namespace",
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "shared-secret"},
							{Name: "my-stateful-set-registry-credentials"},
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
				Name:      "my-stateful-set-registry-credentials",
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
		migrateErr = adoptSecretMigration.Apply(ctx, stSet)
	})

	It("succeeds", func() {
		Expect(migrateErr).NotTo(HaveOccurred())
	})

	It("gets the private registry secret", func() {
		Expect(secretsClient.GetCallCount()).To(Equal(1))
		_, actualNamespace, actualName := secretsClient.GetArgsForCall(0)
		Expect(actualNamespace).To(Equal("my-namespace"))
		Expect(actualName).To(Equal("my-stateful-set-registry-credentials"))
	})

	It("sets the ownership of the registry credentials secret", func() {
		Expect(secretsClient.SetOwnerCallCount()).To(Equal(1))
		_, actualSecret, actualStatefulSet := secretsClient.SetOwnerArgsForCall(0)
		Expect(actualSecret).To(Equal(privateRegistrySecret))
		Expect(actualStatefulSet).To(Equal(stSet))
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

	When("the statefulset does not reference a private registry secret", func() {
		BeforeEach(func() {
			stSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-stateful-set",
					Namespace: "my-namespace",
				},
				Spec: appsv1.StatefulSetSpec{
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

	// It("requests setting the owner of the PDB to the stateful set", func() {
	// 	Expect(pdbClient.SetOwnerCallCount()).To(Equal(1))
	// 	_, actualPDB, actualOwner := pdbClient.SetOwnerArgsForCall(0)
	// 	Expect(actualPDB).To(Equal(pdb))
	// 	Expect(actualOwner).To(Equal(stSet))
	// })

	// When("the stateful set only has a single instance", func() {
	// 	BeforeEach(func() {
	// 		var one int32 = 1
	// 		statefulSet, ok := stSet.(*appsv1.StatefulSet)
	// 		Expect(ok).To(BeTrue())
	// 		statefulSet.Spec.Replicas = &one
	// 	})

	// 	It("does not migrate any PDB as there won't be one", func() {
	// 		Expect(migrateErr).NotTo(HaveOccurred())
	// 		Expect(pdbClient.GetCallCount()).To(BeZero())
	// 		Expect(pdbClient.SetOwnerCallCount()).To(BeZero())
	// 	})
	// })

	// When("a non-statefulset object is received", func() {
	// 	BeforeEach(func() {
	// 		stSet = &appsv1.ReplicaSet{}
	// 	})

	// 	It("errors", func() {
	// 		Expect(migrateErr).To(MatchError("expected *v1.StatefulSet, got: *v1.ReplicaSet"))
	// 	})
	// })

	// When("getting the PDB fails", func() {
	// 	BeforeEach(func() {
	// 		pdbClient.GetReturns(nil, errors.New("no pdb for you"))
	// 	})

	// 	It("bubbles up the error", func() {
	// 		Expect(migrateErr).To(MatchError(ContainSubstring("no pdb for you")))
	// 	})
	// })

	// When("setting the PDB owner fails", func() {
	// 	BeforeEach(func() {
	// 		pdbClient.SetOwnerReturns(nil, errors.New("can't set owner"))
	// 	})

	// 	It("bubbles up the error", func() {
	// 		Expect(migrateErr).To(MatchError(ContainSubstring("can't set owner")))
	// 	})
	// })
})
