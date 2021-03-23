package route_test

import (
	. "code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/k8s/informers/route/routefakes"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
)

var _ = Describe("InstanceChangeInformer", func() {
	var (
		informer      eiriniroute.Informer
		client        kubernetes.Interface
		podWatcher    *watch.FakeWatcher
		updateHandler *routefakes.FakePodUpdateEventHandler
		stopChan      chan struct{}
	)

	setWatcher := func(cs kubernetes.Interface) {
		fakecs, ok := cs.(*fake.Clientset)
		Expect(ok).To(BeTrue())
		podWatcher = watch.NewFake()
		fakecs.PrependWatchReactor("pods", testcore.DefaultWatchReactor(podWatcher, nil))
	}

	BeforeEach(func() {
		updateHandler = new(routefakes.FakePodUpdateEventHandler)
		client = fake.NewSimpleClientset()
		setWatcher(client)

		stopChan = make(chan struct{})

		informer = &InstanceChangeInformer{
			Client:        client,
			Cancel:        stopChan,
			UpdateHandler: updateHandler,
		}
		go informer.Start()
	})

	AfterEach(func() {
		close(stopChan)
	})

	When("a pod gets updated", func() {
		It("should be handled by the update handler", func() {
			pod0 := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mr-stateful-0",
				},
			}
			podWatcher.Add(pod0)

			pod1 := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mr-stateful-0",
				},
				Status: corev1.PodStatus{PodIP: "10.20.30.40"},
			}
			podWatcher.Modify(pod1)

			Eventually(updateHandler.HandleCallCount).Should(Equal(1))
			_, oldPod, newPod := updateHandler.HandleArgsForCall(0)

			Expect(oldPod.Name).To(Equal(newPod.Name))
			Expect(oldPod.Status.PodIP).To(Equal(""))
			Expect(newPod.Status.PodIP).To(Equal("10.20.30.40"))
		})
	})
})
