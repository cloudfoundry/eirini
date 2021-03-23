package stset_test

import (
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("List", func() {
	var (
		logger                    lager.Logger
		statefulSetGetter         *stsetfakes.FakeStatefulSetsBySourceTypeGetter
		statefulsetToLRPConverter *stsetfakes.FakeStatefulSetToLRPConverter

		lister stset.Lister
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-list-statefulset")
		statefulSetGetter = new(stsetfakes.FakeStatefulSetsBySourceTypeGetter)
		statefulsetToLRPConverter = new(stsetfakes.FakeStatefulSetToLRPConverter)

		lister = stset.NewLister(logger, statefulSetGetter, statefulsetToLRPConverter)
	})

	It("translates all existing statefulSets to opi.LRPs", func() {
		st := []appsv1.StatefulSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "odin",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "thor",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "baldur",
				},
			},
		}

		statefulSetGetter.GetBySourceTypeReturns(st, nil)

		Expect(lister.List(ctx)).To(HaveLen(3))
		Expect(statefulsetToLRPConverter.ConvertCallCount()).To(Equal(3))
	})

	It("lists all statefulSets with APP source_type", func() {
		statefulSetGetter.GetBySourceTypeReturns([]appsv1.StatefulSet{}, nil)
		_, err := lister.List(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(statefulSetGetter.GetBySourceTypeCallCount()).To(Equal(1))

		_, sourceType := statefulSetGetter.GetBySourceTypeArgsForCall(0)
		Expect(sourceType).To(Equal("APP"))
	})

	When("no statefulSets exist", func() {
		It("returns an empy list of LRPs", func() {
			statefulSetGetter.GetBySourceTypeReturns([]appsv1.StatefulSet{}, nil)
			Expect(lister.List(ctx)).To(BeEmpty())
			Expect(statefulsetToLRPConverter.ConvertCallCount()).To(Equal(0))
		})
	})

	When("listing statefulsets fails", func() {
		It("should return a meaningful error", func() {
			statefulSetGetter.GetBySourceTypeReturns(nil, errors.New("who is this?"))
			_, err := lister.List(ctx)
			Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
		})
	})
})
