package route_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apps_v1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"

	. "code.cloudfoundry.org/eirini/k8s/route"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("URIChangeInformer", func() {

	const (
		namespace           = "test-me"
		routeMessageTimeout = 600 * time.Millisecond
	)

	var (
		informer    URIChangeInformer
		client      kubernetes.Interface
		watcher     *watch.FakeWatcher
		workChan    chan *route.Message
		stopChan    chan struct{}
		logger      *lagertest.TestLogger
		statefulset *apps_v1.StatefulSet
	)

	createPod := func(name, ip string) {
		p := &v1.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name: name,
				OwnerReferences: []meta.OwnerReference{
					{
						Kind: "StatefulSet",
						Name: "mr-stateful",
					},
				},
				Labels: map[string]string{
					"name": "the-app-name",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{Ports: []v1.ContainerPort{{ContainerPort: 8080}}},
				},
			},
			Status: v1.PodStatus{
				PodIP: ip,
			},
		}

		_, err := client.CoreV1().Pods(namespace).Create(p)
		Expect(err).ToNot(HaveOccurred())
	}

	setWatcher := func(cs kubernetes.Interface) {
		fakecs := cs.(*fake.Clientset)
		watcher = watch.NewFake()
		fakecs.PrependWatchReactor("statefulsets", testcore.DefaultWatchReactor(watcher, nil))
	}

	copyWithModifiedRoute := func(st *apps_v1.StatefulSet, routes string) *apps_v1.StatefulSet {
		thecopy := *st

		thecopy.Annotations = map[string]string{
			"routes": routes,
		}
		return &thecopy
	}

	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		setWatcher(client)

		stopChan = make(chan struct{})
		workChan = make(chan *route.Message, 5)

		logger = lagertest.NewTestLogger("test")
		ctx := lagerctx.NewContext(context.Background(), logger)

		informer = URIChangeInformer{
			Client:     client,
			Cancel:     stopChan,
			Namespace:  namespace,
			SyncPeriod: 0,
			Logger:     lagerctx.FromContext(ctx),
		}
	})

	AfterEach(func() {
		close(stopChan)
	})

	JustBeforeEach(func() {
		go informer.Start(workChan)

		statefulset = &apps_v1.StatefulSet{
			ObjectMeta: meta.ObjectMeta{
				Name: "mr-stateful",
				Annotations: map[string]string{
					"routes": `["mr-stateful.50.60.70.80.nip.io", "mr-boombastic.50.60.70.80.nip.io"]`,
				},
			},
			Spec: apps_v1.StatefulSetSpec{
				Selector: &meta.LabelSelector{
					MatchLabels: map[string]string{
						"name": "the-app-name",
					},
				},
			},
		}
		watcher.Add(statefulset)

		createPod("mr-stateful-0", "10.20.30.40")
		createPod("mr-stateful-1", "50.60.70.80")
	})

	Context("When a new route is added by the user", func() {

		JustBeforeEach(func() {
			newRoutes := `["mr-stateful.50.60.70.80.nip.io", "mr-fantastic.50.60.70.80.nip.io", "mr-boombastic.50.60.70.80.nip.io"]`
			watcher.Modify(copyWithModifiedRoute(statefulset, newRoutes))
		})

		It("should register the new route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name":               Equal("mr-stateful-0"),
				"Routes":             ConsistOf("mr-stateful.50.60.70.80.nip.io", "mr-fantastic.50.60.70.80.nip.io", "mr-boombastic.50.60.70.80.nip.io"),
				"UnregisteredRoutes": BeEmpty(),
				"InstanceID":         Equal("mr-stateful-0"),
				"Address":            Equal("10.20.30.40"),
				"Port":               BeNumerically("==", 8080),
				"TLSPort":            BeNumerically("==", 0),
			}))))
		})

		It("should register the new route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name":               Equal("mr-stateful-1"),
				"Routes":             ConsistOf("mr-stateful.50.60.70.80.nip.io", "mr-fantastic.50.60.70.80.nip.io", "mr-boombastic.50.60.70.80.nip.io"),
				"UnregisteredRoutes": BeEmpty(),
				"InstanceID":         Equal("mr-stateful-1"),
				"Address":            Equal("50.60.70.80"),
				"Port":               BeNumerically("==", 8080),
				"TLSPort":            BeNumerically("==", 0),
			}))))
		})

	})

	Context("When a route is removed by the user", func() {

		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `["mr-stateful.50.60.70.80.nip.io"]`))
		})

		It("should unregister the deleted route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name":               Equal("mr-stateful-0"),
				"Routes":             ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
				"InstanceID":         Equal("mr-stateful-0"),
				"Address":            Equal("10.20.30.40"),
				"Port":               BeNumerically("==", 8080),
				"TLSPort":            BeNumerically("==", 0),
			}))))
		})

		It("should unregister the deleted route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name":               Equal("mr-stateful-1"),
				"Routes":             ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
				"InstanceID":         Equal("mr-stateful-1"),
				"Address":            Equal("50.60.70.80"),
				"Port":               BeNumerically("==", 8080),
				"TLSPort":            BeNumerically("==", 0),
			}))))
		})
	})

	Context("When the pods cannot be listed", func() {

		BeforeEach(func() {
			reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("boom")
			}
			informer.Client.(*fake.Clientset).PrependReactor("list", "pods", reaction)
		})

		JustBeforeEach(func() {
			newRoutes := `["shaggy.50.60.70.80.nip.io"]`
			watcher.Modify(copyWithModifiedRoute(statefulset, newRoutes))
		})

		It("should not send any routes", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive())
		})

		It("should print an error", func() {
			Eventually(logger.LogMessages, routeMessageTimeout).Should(ContainElement("test.failed-to-get-child-pods"))
		})
	})

	Context("When the app is deleted", func() {

		JustBeforeEach(func() {
			watcher.Delete(statefulset)
		})

		It("should unregister all routes for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name":               Equal("mr-stateful-0"),
				"Routes":             BeEmpty(),
				"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io", "mr-stateful.50.60.70.80.nip.io"),
				"InstanceID":         Equal("mr-stateful-0"),
				"Address":            Equal("10.20.30.40"),
				"Port":               BeNumerically("==", 8080),
				"TLSPort":            BeNumerically("==", 0),
			}))))

		})

		It("should unregister all routes for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name":               Equal("mr-stateful-1"),
				"Routes":             BeEmpty(),
				"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io", "mr-stateful.50.60.70.80.nip.io"),
				"InstanceID":         Equal("mr-stateful-1"),
				"Address":            Equal("50.60.70.80"),
				"Port":               BeNumerically("==", 8080),
				"TLSPort":            BeNumerically("==", 0),
			}))))

		})

	})
})
