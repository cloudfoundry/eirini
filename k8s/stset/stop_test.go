package stset_test

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Stop", func() {
	var (
		logger             lager.Logger
		statefulSetGetter  *stsetfakes.FakeStatefulSetByLRPIdentifierGetter
		statefulSetDeleter *stsetfakes.FakeStatefulSetDeleter
		podDeleter         *stsetfakes.FakePodDeleter
		pdbDeleter         *stsetfakes.FakePodDisruptionBudgetDeleter
		secretsDeleter     *stsetfakes.FakeSecretsDeleter

		stopper stset.Stopper
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-stop-statefulset")
		statefulSetGetter = new(stsetfakes.FakeStatefulSetByLRPIdentifierGetter)
		statefulSetDeleter = new(stsetfakes.FakeStatefulSetDeleter)
		podDeleter = new(stsetfakes.FakePodDeleter)
		pdbDeleter = new(stsetfakes.FakePodDisruptionBudgetDeleter)
		secretsDeleter = new(stsetfakes.FakeSecretsDeleter)

		stopper = stset.NewStopper(logger, statefulSetGetter, statefulSetDeleter, podDeleter, pdbDeleter, secretsDeleter)
	})

	Describe("Stop StatefulSet", func() {
		var statefulSets []appsv1.StatefulSet

		BeforeEach(func() {
			statefulSets = []appsv1.StatefulSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baldur",
						Namespace: "the-namespace",
					},
				},
			}
			statefulSetGetter.GetByLRPIdentifierReturns(statefulSets, nil)
			pdbDeleter.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{
				Group:    "policy/v1beta1",
				Resource: "PodDisruptionBudet",
			},
				"foo"))
		})

		It("deletes the statefulSet", func() {
			Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
			Expect(statefulSetDeleter.DeleteCallCount()).To(Equal(1))
			_, namespace, name := statefulSetDeleter.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(name).To(Equal("baldur"))
		})

		It("should delete any corresponding pod disruption budgets", func() {
			Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
			Expect(pdbDeleter.DeleteCallCount()).To(Equal(1))
			_, namespace, pdbName := pdbDeleter.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(pdbName).To(Equal("baldur"))
		})

		When("the stateful set runs an image from a private registry", func() {
			BeforeEach(func() {
				statefulSets[0].Spec = appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "baldur-registry-credentials"},
							},
						},
					},
				}
			})

			It("deletes the secret holding the creds of the private registry", func() {
				Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				Expect(secretsDeleter.DeleteCallCount()).To(Equal(1))
				_, secretNs, secretName := secretsDeleter.DeleteArgsForCall(0)
				Expect(secretName).To(Equal("baldur-registry-credentials"))
				Expect(secretNs).To(Equal("the-namespace"))
			})

			When("deleting the private registry secret fails", func() {
				BeforeEach(func() {
					secretsDeleter.DeleteReturns(errors.New("boom"))
				})

				It("returns the error", func() {
					Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(MatchError(ContainSubstring("boom")))
				})
			})

			When("the private registry secret does not exist", func() {
				BeforeEach(func() {
					secretsDeleter.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{
						Group:    "core/v1",
						Resource: "Secret",
					}, "foo"))
				})

				It("succeeds", func() {
					Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				})
			})
		})

		When("deletion of stateful set fails", func() {
			BeforeEach(func() {
				statefulSetDeleter.DeleteReturns(errors.New("boom"))
			})

			It("should return a meaningful error", func() {
				Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to delete statefulset")))
			})
		})

		When("deletion of stateful set conflicts", func() {
			It("should retry", func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}}, nil)
				statefulSetDeleter.DeleteReturnsOnCall(0, k8serrors.NewConflict(schema.GroupResource{}, "foo", errors.New("boom")))
				statefulSetDeleter.DeleteReturnsOnCall(1, nil)
				Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				Expect(statefulSetDeleter.DeleteCallCount()).To(Equal(2))
			})
		})

		When("pdb deletion fails", func() {
			It("returns an error", func() {
				pdbDeleter.DeleteReturns(errors.New("boom"))

				Expect(stopper.Stop(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(MatchError(ContainSubstring("boom")))
			})
		})

		When("kubernetes fails to list statefulsets", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("who is this?"))
			})

			It("should return a meaningful error", func() {
				Expect(stopper.Stop(ctx, opi.LRPIdentifier{})).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("the statefulSet does not exist", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
			})

			It("succeeds", func() {
				Expect(stopper.Stop(ctx, opi.LRPIdentifier{})).To(Succeed())
			})

			It("logs useful information", func() {
				_ = stopper.Stop(ctx, opi.LRPIdentifier{GUID: "missing_guid", Version: "some_version"})
				Expect(logger).To(gbytes.Say("statefulset-does-not-exist.*missing_guid.*some_version"))
			})
		})
	})

	Describe("StopInstance", func() {
		BeforeEach(func() {
			st := []appsv1.StatefulSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baldur-space-foo-34f869d015",
						Namespace: "the-namespace",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32ptr(2),
					},
				},
			}

			statefulSetGetter.GetByLRPIdentifierReturns(st, nil)
		})

		It("deletes a pod instance", func() {
			Expect(stopper.StopInstance(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 0)).
				To(Succeed())

			Expect(podDeleter.DeleteCallCount()).To(Equal(1))

			_, namespace, name := podDeleter.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(name).To(Equal("baldur-space-foo-34f869d015-0"))
		})

		When("there's an internal K8s error", func() {
			It("should return an error", func() {
				statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("boom"))
				Expect(stopper.StopInstance(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("the statefulset does not exist", func() {
			It("succeeds", func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
				Expect(stopper.StopInstance(ctx, opi.LRPIdentifier{GUID: "some", Version: "thing"}, 1)).To(Succeed())
			})
		})

		When("the instance index is invalid", func() {
			It("returns an error", func() {
				podDeleter.DeleteReturns(errors.New("boom"))
				Expect(stopper.StopInstance(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 42)).
					To(MatchError(eirini.ErrInvalidInstanceIndex))
			})
		})

		When("the instance is already stopped", func() {
			BeforeEach(func() {
				podDeleter.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{}, "potato"))
			})

			It("succeeds", func() {
				Expect(stopper.StopInstance(ctx, opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(Succeed())
			})
		})
	})
})

func int32ptr(i int) *int32 {
	u := int32(i)

	return &u
}
