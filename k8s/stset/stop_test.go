package stset_test

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
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

		stopper stset.Stopper
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-stop-statefulset")
		statefulSetGetter = new(stsetfakes.FakeStatefulSetByLRPIdentifierGetter)
		statefulSetDeleter = new(stsetfakes.FakeStatefulSetDeleter)
		podDeleter = new(stsetfakes.FakePodDeleter)

		stopper = stset.NewStopper(logger, statefulSetGetter, statefulSetDeleter, podDeleter)
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
		})

		It("deletes the statefulSet", func() {
			Expect(stopper.Stop(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
			Expect(statefulSetDeleter.DeleteCallCount()).To(Equal(1))
			_, namespace, name := statefulSetDeleter.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(name).To(Equal("baldur"))
		})

		When("deletion of stateful set fails", func() {
			BeforeEach(func() {
				statefulSetDeleter.DeleteReturns(errors.New("boom"))
			})

			It("should return a meaningful error", func() {
				Expect(stopper.Stop(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to delete statefulset")))
			})
		})

		When("deletion of stateful set conflicts", func() {
			It("should retry", func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}}, nil)
				statefulSetDeleter.DeleteReturnsOnCall(0, k8serrors.NewConflict(schema.GroupResource{}, "foo", errors.New("boom")))
				statefulSetDeleter.DeleteReturnsOnCall(1, nil)
				Expect(stopper.Stop(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				Expect(statefulSetDeleter.DeleteCallCount()).To(Equal(2))
			})
		})

		When("kubernetes fails to list statefulsets", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("who is this?"))
			})

			It("should return a meaningful error", func() {
				Expect(stopper.Stop(ctx, api.LRPIdentifier{})).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("the statefulSet does not exist", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
			})

			It("succeeds", func() {
				Expect(stopper.Stop(ctx, api.LRPIdentifier{})).To(Succeed())
			})

			It("logs useful information", func() {
				_ = stopper.Stop(ctx, api.LRPIdentifier{GUID: "missing_guid", Version: "some_version"})
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
			Expect(stopper.StopInstance(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 0)).
				To(Succeed())

			Expect(podDeleter.DeleteCallCount()).To(Equal(1))

			_, namespace, name := podDeleter.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(name).To(Equal("baldur-space-foo-34f869d015-0"))
		})

		When("there's an internal K8s error", func() {
			It("should return an error", func() {
				statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("boom"))
				Expect(stopper.StopInstance(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("the statefulset does not exist", func() {
			It("succeeds", func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
				Expect(stopper.StopInstance(ctx, api.LRPIdentifier{GUID: "some", Version: "thing"}, 1)).To(Succeed())
			})
		})

		When("the instance index is invalid", func() {
			It("returns an error", func() {
				podDeleter.DeleteReturns(errors.New("boom"))
				Expect(stopper.StopInstance(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 42)).
					To(MatchError(eirini.ErrInvalidInstanceIndex))
			})
		})

		When("the instance is already stopped", func() {
			BeforeEach(func() {
				podDeleter.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{}, "potato"))
			})

			It("succeeds", func() {
				Expect(stopper.StopInstance(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(Succeed())
			})
		})
	})
})

func int32ptr(i int) *int32 {
	u := int32(i)

	return &u
}
