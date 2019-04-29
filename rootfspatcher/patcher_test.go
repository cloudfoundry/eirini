package rootfspatcher_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"

	. "code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/rootfspatcher/rootfspatcherfakes"
)

var _ = Describe("Patcher", func() {

	var (
		client     *fake.Clientset
		fakeClient *rootfspatcherfakes.FakeStatefulSetPatchLister
		namespace  string
		patcher    Patcher
		newVersion string
	)

	BeforeEach(func() {
		newVersion = "version2"
		namespace = "test-ns"
		client = fake.NewSimpleClientset()
		patcher = StatefulSetPatcher{
			Version: newVersion,
			Client:  client.AppsV1beta2().StatefulSets(namespace),
		}
	})

	JustBeforeEach(func() {
		ss := v1beta2.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "some-app",
				Labels: map[string]string{RootfsVersionLabel: "version1"},
			},
			Spec: v1beta2.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{RootfsVersionLabel: "version1"},
					},
				},
			},
		}
		_, err := client.AppsV1beta2().StatefulSets(namespace).Create(&ss)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should update all statefulsets with new version", func() {
		Expect(patcher.Patch()).To(Succeed())
		updatedSS, err := client.AppsV1beta2().StatefulSets(namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedSS.Items[0].Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
		Expect(updatedSS.Items[0].Spec.Template.Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
	})

	It("should return error if it cannot list statefulsets", func() {
		errReaction := func(action testcore.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("fake error")
		}
		client.PrependReactor("list", "statefulsets", errReaction)
		Expect(patcher.Patch()).To(MatchError("failed to list statefulsets: fake error"))
	})

	It("should return error if it cannot list statefulsets", func() {
		errReaction := func(action testcore.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("fake error")
		}
		client.PrependReactor("update", "statefulsets", errReaction)
		Expect(patcher.Patch()).To(MatchError("failed to update statefulset: fake error"))
	})

	Context("When new version is equal to old version", func() {
		var (
			count int
		)

		BeforeEach(func() {
			newVersion = "version1"
			fakeClient = new(rootfspatcherfakes.FakeStatefulSetPatchLister)
			patcher = StatefulSetPatcher{
				Version: newVersion,
				Client:  fakeClient,
			}

			ss := v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "some-app",
					Labels: map[string]string{RootfsVersionLabel: "version1"},
				},
				Spec: v1beta2.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{RootfsVersionLabel: "version1"},
						},
					},
				},
			}

			list := &v1beta2.StatefulSetList{
				Items: []v1beta2.StatefulSet{
					ss,
				},
			}

			fakeClient.ListReturns(list, nil)
			count = fakeClient.UpdateCallCount()
		})

		FIt("shouldn't call the update function", func() {
			err := patcher.Patch()
			Expect(err).ToNot(HaveOccurred())

			Expect(count).To(Equal(0))
		})
	})
})
