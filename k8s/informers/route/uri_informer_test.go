package route_test

import (
	. "code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/k8s/informers/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
)

var _ = Describe("URIChangeInformer", func() {
	var (
		informer      URIChangeInformer
		client        kubernetes.Interface
		watcher       *watch.FakeWatcher
		updateHandler *routefakes.FakeStatefulSetUpdateEventHandler
		deleteHandler *routefakes.FakeStatefulSetDeleteEventHandler
		stopChan      chan struct{}
	)

	setWatcher := func(cs kubernetes.Interface) {
		fakecs, ok := cs.(*fake.Clientset)
		Expect(ok).To(BeTrue())
		watcher = watch.NewFake()
		fakecs.PrependWatchReactor("statefulsets", testcore.DefaultWatchReactor(watcher, nil))
	}

	BeforeEach(func() {
		updateHandler = new(routefakes.FakeStatefulSetUpdateEventHandler)
		deleteHandler = new(routefakes.FakeStatefulSetDeleteEventHandler)
		client = fake.NewSimpleClientset()
		setWatcher(client)

		stopChan = make(chan struct{})

		informer = URIChangeInformer{
			Client:        client,
			Cancel:        stopChan,
			UpdateHandler: updateHandler,
			DeleteHandler: deleteHandler,
		}
		go informer.Start()
	})

	AfterEach(func() {
		close(stopChan)
	})

	When("a statefulset gets updated", func() {
		BeforeEach(func() {
			statefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mr-stateful",
					Annotations: map[string]string{
						"somewhere": "over",
					},
				},
			}
			watcher.Add(statefulSet)

			updatedStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mr-stateful",
					Annotations: map[string]string{
						"the": "rainbow",
					},
				},
			}
			watcher.Modify(updatedStatefulSet)
		})

		It("should be handled by the update handler", func() {
			Eventually(updateHandler.HandleCallCount).Should(Equal(1))

			_, oldStatefulSet, updatedStatefulSet := updateHandler.HandleArgsForCall(0)

			Expect(oldStatefulSet.Name).To(Equal(updatedStatefulSet.Name))
			Expect(oldStatefulSet.Annotations).To(HaveKeyWithValue("somewhere", "over"))
			Expect(updatedStatefulSet.Annotations).To(HaveKeyWithValue("the", "rainbow"))
		})
	})

	When("a statefulset gets deleted", func() {
		BeforeEach(func() {
			statefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mr-stateful",
					Annotations: map[string]string{
						"somewhere": "over",
					},
				},
			}
			watcher.Add(statefulSet)
			watcher.Delete(statefulSet)
		})

		It("should be handled by the update handler", func() {
			Eventually(deleteHandler.HandleCallCount).Should(Equal(1))

			_, deletedStatefulSet := deleteHandler.HandleArgsForCall(0)

			Expect(deletedStatefulSet.Name).To(Equal("mr-stateful"))
			Expect(deletedStatefulSet.Annotations).To(HaveKeyWithValue("somewhere", "over"))
		})
	})
})
