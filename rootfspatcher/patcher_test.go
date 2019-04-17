package rootfspatcher_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/apps/v1beta2"
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
		namespace  string
		patcher    Patcher
		waiter     *rootfspatcherfakes.FakeWaiter
		newVersion string
	)
	BeforeEach(func() {
		namespace = "test-ns"
		client = fake.NewSimpleClientset()
		ss := v1beta2.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "some-app",
				Labels: map[string]string{RootfsVersionLabel: "version1"},
			},
		}
		client.AppsV1beta2().StatefulSets(namespace).Create(&ss)

		waiter = &rootfspatcherfakes.FakeWaiter{}

		newVersion = "version2"
		patcher = Patcher{
			Version: newVersion,
			Client:  client.AppsV1beta2().StatefulSets(namespace),
			Waiter:  waiter,
		}
	})

	It("should update all statefulsets with new version", func() {
		patcher.Patch()
		updatedSS, err := client.AppsV1beta2().StatefulSets(namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedSS.Items[0].Labels).To(HaveKeyWithValue(RootfsVersionLabel, newVersion))
	})

	It("should wait for all pods to become running", func() {
		patcher.Patch()
		Expect(waiter.WaitCallCount()).To(Equal(1))
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

	It("should return error if the waiter returns an error", func() {
		waiter.WaitReturns(errors.New("cannot wait"))
		Expect(patcher.Patch()).To(MatchError("failed to wait for update: cannot wait"))
	})

})
