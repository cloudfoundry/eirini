package rootfspatcher_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/rootfspatcher/rootfspatcherfakes"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Patcher", func() {

	var (
		statefulsetUpdaterLister *rootfspatcherfakes.FakeStatefulSetUpdaterLister
		patcher                  StatefulSetPatcher
		newVersion               string
		logger                   *lagertest.TestLogger
		stsList                  *appsv1.StatefulSetList
	)

	BeforeEach(func() {
		newVersion = "version1"
		logger = lagertest.NewTestLogger("test")
		statefulsetUpdaterLister = new(rootfspatcherfakes.FakeStatefulSetUpdaterLister)
		patcher = StatefulSetPatcher{
			Version:      newVersion,
			StatefulSets: statefulsetUpdaterLister,
			Logger:       logger,
		}

		stsList = &appsv1.StatefulSetList{
			Items: []appsv1.StatefulSet{
				createStatefulSet("some-app", "version1"),
			},
		}

		statefulsetUpdaterLister.ListReturns(stsList, nil)
	})

	Context("When a new version is provided", func() {
		It("should succeed", func() {
			Expect(patcher.Patch()).To(Succeed())
		})

		It("should call the updated function once", func() {
			Expect(patcher.Patch()).To(Succeed())
			Expect(statefulsetUpdaterLister.UpdateCallCount()).To(Equal(1))
		})

		It("should update all statefulsets labels with the new version", func() {
			Expect(patcher.Patch()).To(Succeed())
			_, updatedStatefulset, _ := statefulsetUpdaterLister.UpdateArgsForCall(0)
			Expect(updatedStatefulset.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
		})

		It("should update all statefulsets template specs with the new version", func() {
			Expect(patcher.Patch()).To(Succeed())
			_, updatedStatefulset, _ := statefulsetUpdaterLister.UpdateArgsForCall(0)
			Expect(updatedStatefulset.Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
		})

		Context("When Patch fails", func() {
			Context("because it cannot list statefulsets", func() {
				It("should fail with a meaningful error message", func() {
					statefulsetUpdaterLister.ListReturns(&appsv1.StatefulSetList{}, errors.New("arrrgh"))
					Expect(patcher.Patch()).To(MatchError(ContainSubstring("failed to list statefulset")))
				})
			})

			Context("because it cannot update statefulsets", func() {
				It("should fail with a meaningful error message", func() {
					statefulsetUpdaterLister.UpdateReturns(&appsv1.StatefulSet{}, errors.New("brrrgh"))
					Expect(patcher.Patch()).To(MatchError(ContainSubstring("failed to update 1 statefulsets")))
				})
			})
		})

		Context("When an additional statefulset exists", func() {
			BeforeEach(func() {
				stsList.Items = append(stsList.Items, createStatefulSet("another-app", "version2"))
			})

			It("should call the updated function once", func() {
				Expect(patcher.Patch()).To(Succeed())
				Expect(statefulsetUpdaterLister.UpdateCallCount()).To(Equal(2))
			})

			It("should update all statefulsets labels with the new version", func() {
				Expect(patcher.Patch()).To(Succeed())
				_, updatedStatefulset, _ := statefulsetUpdaterLister.UpdateArgsForCall(0)
				_, updatedStatefulset2, _ := statefulsetUpdaterLister.UpdateArgsForCall(1)

				Expect(updatedStatefulset.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
				Expect(updatedStatefulset2.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
			})

			It("should update all statefulsets template specs with the new version", func() {
				Expect(patcher.Patch()).To(Succeed())
				_, updatedStatefulset, _ := statefulsetUpdaterLister.UpdateArgsForCall(0)
				_, updatedStatefulset2, _ := statefulsetUpdaterLister.UpdateArgsForCall(1)

				Expect(updatedStatefulset.Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
				Expect(updatedStatefulset2.Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
			})

			Context("When Patch fails with multiple statefulsets", func() {
				It("should fail with a meaningful error message", func() {
					statefulsetUpdaterLister.UpdateReturns(&appsv1.StatefulSet{}, errors.New("brrrgh"))
					Expect(patcher.Patch()).To(MatchError(ContainSubstring("failed to update 2 statefulsets")))
				})
			})
		})
	})
})

func createStatefulSet(name, version string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{RootfsVersionLabel: version},
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RootfsVersionLabel: version},
				},
			},
		},
	}
}
