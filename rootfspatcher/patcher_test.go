package rootfspatcher_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/rootfspatcher/rootfspatcherfakes"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Patcher", func() {

	var (
		statefulsetUpdaterLister *rootfspatcherfakes.FakeStatefulSetUpdaterLister
		patcher                  Patcher
		newVersion               string
		logger                   *lagertest.TestLogger
		err                      error
		stsList                  *v1beta2.StatefulSetList
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

		stsList = &v1beta2.StatefulSetList{
			Items: []v1beta2.StatefulSet{
				createStatefulSet("some-app", "version1"),
			},
		}

		statefulsetUpdaterLister.ListReturns(stsList, nil)
	})

	JustBeforeEach(func() {
		err = patcher.Patch()
	})

	Context("When a new version is provided", func() {

		It("should succeed", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the updated function once", func() {
			Expect(statefulsetUpdaterLister.UpdateCallCount()).To(Equal(1))
		})

		It("should update all statefulsets labels with the new version", func() {
			updatedStatefulset := statefulsetUpdaterLister.UpdateArgsForCall(0)
			Expect(updatedStatefulset.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
		})

		It("should update all statefulsets template specs with the new version", func() {
			updatedStatefulset := statefulsetUpdaterLister.UpdateArgsForCall(0)
			Expect(updatedStatefulset.Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
		})

		It("should log a line for each statefulset update", func() {

		})

		Context("When Patch fails", func() {
			Context("because it cannot list statefulsets", func() {
				BeforeEach(func() {
					statefulsetUpdaterLister.ListReturns(&v1beta2.StatefulSetList{}, errors.New("arrrgh"))
				})

				It("should fail with a meaningful error message", func() {
					Expect(err).To(MatchError(ContainSubstring("failed to list statefulset")))
				})
			})

			Context("because it cannot update statefulsets", func() {
				BeforeEach(func() {
					statefulsetUpdaterLister.UpdateReturns(&v1beta2.StatefulSet{}, errors.New("brrrgh"))
				})

				It("should fail with a meaningful error message", func() {
					Expect(err).To(MatchError(ContainSubstring("failed to update 1 statefulsets")))
				})
			})
		})

		Context("When an additional statefulset exists", func() {
			BeforeEach(func() {
				stsList.Items = append(stsList.Items, createStatefulSet("another-app", "version2"))
			})

			It("should succeed", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should call the updated function once", func() {
				Expect(statefulsetUpdaterLister.UpdateCallCount()).To(Equal(2))
			})

			It("should update all statefulsets labels with the new version", func() {
				updatedStatefulset := statefulsetUpdaterLister.UpdateArgsForCall(0)
				updatedStatefulset2 := statefulsetUpdaterLister.UpdateArgsForCall(1)

				Expect(updatedStatefulset.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
				Expect(updatedStatefulset2.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
			})

			It("should update all statefulsets template specs with the new version", func() {
				updatedStatefulset := statefulsetUpdaterLister.UpdateArgsForCall(0)
				updatedStatefulset2 := statefulsetUpdaterLister.UpdateArgsForCall(1)

				Expect(updatedStatefulset.Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
				Expect(updatedStatefulset2.Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
			})

			Context("When Patch fails with multiple statefulsets", func() {
				BeforeEach(func() {
					statefulsetUpdaterLister.UpdateReturns(&v1beta2.StatefulSet{}, errors.New("brrrgh"))
				})

				It("should fail with a meaningful error message", func() {
					Expect(err).To(MatchError(ContainSubstring("failed to update 2 statefulsets")))
				})
			})
		})
	})
})

func createStatefulSet(name, version string) v1beta2.StatefulSet {
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{RootfsVersionLabel: version},
		},
		Spec: v1beta2.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RootfsVersionLabel: version},
				},
			},
		},
	}
}
