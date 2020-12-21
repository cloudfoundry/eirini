package stset_test

import (
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Update", func() {
	var (
		logger             lager.Logger
		statefulSetGetter  *stsetfakes.FakeStatefulSetByLRPIdentifierGetter
		statefulSetUpdater *stsetfakes.FakeStatefulSetUpdater
		pdbDeleter         *stsetfakes.FakePodDisruptionBudgetDeleter
		pdbCreator         *stsetfakes.FakePodDisruptionBudgetCreator

		updatedLRP *opi.LRP
		err        error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("handler-test")

		statefulSetGetter = new(stsetfakes.FakeStatefulSetByLRPIdentifierGetter)
		statefulSetUpdater = new(stsetfakes.FakeStatefulSetUpdater)
		pdbDeleter = new(stsetfakes.FakePodDisruptionBudgetDeleter)
		pdbCreator = new(stsetfakes.FakePodDisruptionBudgetCreator)

		updatedLRP = &opi.LRP{
			LRPIdentifier: opi.LRPIdentifier{
				GUID:    "guid_1234",
				Version: "version_1234",
			},
			AppName:         "baldur",
			SpaceName:       "space-foo",
			TargetInstances: 5,
			LastUpdated:     "now",
			AppURIs:         []opi.Route{{Hostname: "new-route.io", Port: 6666}},
			Image:           "new/image",
		}

		replicas := int32(3)

		st := []appsv1.StatefulSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baldur",
					Namespace: "the-namespace",
					Annotations: map[string]string{
						stset.AnnotationProcessGUID:      "Baldur-guid",
						stset.AnnotationLastUpdated:      "never",
						stset.AnnotationRegisteredRoutes: `[{"hostname":"myroute.io","port":1000}]`,
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "another-container", Image: "another/image"},
								{Name: stset.OPIContainerName, Image: "old/image"},
							},
						},
					},
				},
			},
		}

		statefulSetGetter.GetByLRPIdentifierReturns(st, nil)
	})

	JustBeforeEach(func() {
		updater := stset.NewUpdater(logger, statefulSetGetter, statefulSetUpdater, pdbDeleter, pdbCreator)
		err = updater.Update(updatedLRP)
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("updates the statefulset", func() {
		Expect(statefulSetUpdater.UpdateCallCount()).To(Equal(1))

		namespace, st := statefulSetUpdater.UpdateArgsForCall(0)
		Expect(namespace).To(Equal("the-namespace"))
		Expect(st.GetAnnotations()).To(HaveKeyWithValue(stset.AnnotationLastUpdated, "now"))
		Expect(st.GetAnnotations()).To(HaveKeyWithValue(stset.AnnotationRegisteredRoutes, `[{"hostname":"new-route.io","port":6666}]`))
		Expect(st.GetAnnotations()).NotTo(HaveKey("another"))
		Expect(*st.Spec.Replicas).To(Equal(int32(5)))
		Expect(st.Spec.Template.Spec.Containers[0].Image).To(Equal("another/image"))
		Expect(st.Spec.Template.Spec.Containers[1].Image).To(Equal("new/image"))
	})

	When("the image is missing", func() {
		BeforeEach(func() {
			updatedLRP.Image = ""
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("doesn't reset the image", func() {
			Expect(statefulSetUpdater.UpdateCallCount()).To(Equal(1))

			_, st := statefulSetUpdater.UpdateArgsForCall(0)
			Expect(st.Spec.Template.Spec.Containers[1].Image).To(Equal("old/image"))
		})
	})

	When("lrp is scaled down to 1 instance", func() {
		BeforeEach(func() {
			updatedLRP.TargetInstances = 1
		})

		It("should delete the pod disruption budget for the lrp", func() {
			Expect(pdbDeleter.DeleteCallCount()).To(Equal(1))
			pdbNamespace, pdbName := pdbDeleter.DeleteArgsForCall(0)
			Expect(pdbNamespace).To(Equal("the-namespace"))
			Expect(pdbName).To(Equal("baldur"))
		})

		When("the pod disruption budget does not exist", func() {
			BeforeEach(func() {
				pdbDeleter.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{
					Group:    "policy/v1beta1",
					Resource: "PodDisruptionBudget",
				}, "baldur"))
			})

			It("should ignore the error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the pod disruption budget deletion errors", func() {
			BeforeEach(func() {
				pdbDeleter.DeleteReturns(errors.New("pow"))
			})

			It("should propagate the error", func() {
				Expect(err).To(MatchError(ContainSubstring("pow")))
			})
		})
	})

	When("lrp is scaled up to more than 1 instance", func() {
		BeforeEach(func() {
			updatedLRP.TargetInstances = 2
		})

		It("should create a pod disruption budget for the lrp in the same namespace", func() {
			Expect(pdbCreator.CreateCallCount()).To(Equal(1))
			namespace, pdb := pdbCreator.CreateArgsForCall(0)

			Expect(pdb.Name).To(Equal("baldur"))
			Expect(namespace).To(Equal("the-namespace"))
		})

		When("the pod disruption budget already exists", func() {
			BeforeEach(func() {
				pdbCreator.CreateReturns(nil, k8serrors.NewAlreadyExists(schema.GroupResource{
					Group:    "policy/v1beta1",
					Resource: "PodDisruptionBudget",
				}, "baldur"))
			})

			It("should ignore the error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the pod disruption budget creation errors", func() {
			BeforeEach(func() {
				pdbCreator.CreateReturns(nil, errors.New("boom"))
			})

			It("should propagate the error", func() {
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})
	})

	When("update fails", func() {
		BeforeEach(func() {
			statefulSetUpdater.UpdateReturns(nil, errors.New("boom"))
		})

		It("should return a meaningful message", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to update statefulset")))
		})
	})

	When("update fails because of a conflict", func() {
		BeforeEach(func() {
			statefulSetUpdater.UpdateReturnsOnCall(0, nil, k8serrors.NewConflict(schema.GroupResource{}, "foo", errors.New("boom")))
			statefulSetUpdater.UpdateReturnsOnCall(1, &appsv1.StatefulSet{}, nil)
		})

		It("should retry", func() {
			Expect(statefulSetUpdater.UpdateCallCount()).To(Equal(2))
		})
	})

	When("the app does not exist", func() {
		BeforeEach(func() {
			statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("sorry"))
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
		})

		It("should not create the app", func() {
			Expect(statefulSetUpdater.UpdateCallCount()).To(Equal(0))
		})
	})
})
