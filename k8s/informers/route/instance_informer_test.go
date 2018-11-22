package route_test

import (
	"context"
	"time"

	. "code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/route"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apps_v1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("InstanceChangeInformer", func() {

	const (
		namespace           = "test-me"
		routeMessageTimeout = 600 * time.Millisecond
	)

	var (
		informer   route.Informer
		client     kubernetes.Interface
		podWatcher *watch.FakeWatcher
		workChan   chan *route.Message
		stopChan   chan struct{}
		logger     *lagertest.TestLogger
		pod0       *v1.Pod
		pod1       *v1.Pod
	)

	setWatcher := func(cs kubernetes.Interface) {
		fakecs := cs.(*fake.Clientset)
		podWatcher = watch.NewFake()
		fakecs.PrependWatchReactor("pods", testcore.DefaultWatchReactor(podWatcher, nil))
	}

	createPod := func(name string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name: name,
				OwnerReferences: []meta.OwnerReference{
					{
						Kind: "StatefulSet",
						Name: "mr-stateful",
					},
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{Ports: []v1.ContainerPort{{ContainerPort: 8080}}},
				},
			},
		}
	}

	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		setWatcher(client)

		stopChan = make(chan struct{})
		workChan = make(chan *route.Message, 5)

		logger = lagertest.NewTestLogger("test")
		ctx := lagerctx.NewContext(context.Background(), logger)

		informer = &InstanceChangeInformer{
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

		st := &apps_v1.StatefulSet{
			ObjectMeta: meta.ObjectMeta{
				Name: "mr-stateful",
				Annotations: map[string]string{
					"routes": `["mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"]`,
				},
			},
		}
		_, err := client.AppsV1().StatefulSets(namespace).Create(st)
		Expect(err).ToNot(HaveOccurred())

		podWatcher.Add(pod0)
		podWatcher.Add(pod1)
	})

	Context("When a updated pod is missing its IP", func() {

		BeforeEach(func() {
			pod0 = createPod("mr-stateful-0")
			pod1 = createPod("mr-stateful-1")
		})

		JustBeforeEach(func() {
			pod0.Status = v1.PodStatus{Message: "where my IP at?"}
			podWatcher.Modify(pod0)
			pod1.Status = v1.PodStatus{PodIP: "50.60.70.80"}
			podWatcher.Modify(pod1)
		})

		It("should not send a route for the pod", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
				"Name": Equal("mr-stateful-0"),
			}))))
		})

		It("should not prevent other routes to be sent", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Name:       "mr-stateful-1",
				Routes:     []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
				InstanceID: "mr-stateful-1",
				Address:    "50.60.70.80",
				Port:       8080,
				TLSPort:    0,
			})))
		})
	})

	Context("When a updated pod is missing its port", func() {

		BeforeEach(func() {
			pod0 = createPod("mr-stateful-0")
			pod1 = createPod("mr-stateful-1")
		})

		JustBeforeEach(func() {
			pod0.Status = v1.PodStatus{Message: "where my port at?"}
			pod0.Spec.Containers[0].Ports = []v1.ContainerPort{}
			podWatcher.Modify(pod0)
			pod1.Status = v1.PodStatus{PodIP: "50.60.70.80"}
			podWatcher.Modify(pod1)
		})

		It("should not send a route for the pod", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
				"Name": Equal("mr-stateful-0"),
			}))))
		})

		It("should not prevent other routes to be sent", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Name:       "mr-stateful-1",
				Routes:     []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
				InstanceID: "mr-stateful-1",
				Address:    "50.60.70.80",
				Port:       8080,
				TLSPort:    0,
			})))
		})
	})

	Context("When an ip is assigned to pods", func() {

		BeforeEach(func() {
			pod0 = createPod("mr-stateful-0")
			pod1 = createPod("mr-stateful-1")
		})

		JustBeforeEach(func() {
			pod0.Status = v1.PodStatus{PodIP: "10.20.30.40"}
			podWatcher.Modify(pod0)
			pod1.Status = v1.PodStatus{PodIP: "50.60.70.80"}
			podWatcher.Modify(pod1)
		})

		It("should send the first route", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Name:       "mr-stateful-0",
				Routes:     []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       8080,
				TLSPort:    0,
			})))
		})

		It("should send the second route", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Name:       "mr-stateful-1",
				Routes:     []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
				InstanceID: "mr-stateful-1",
				Address:    "50.60.70.80",
				Port:       8080,
				TLSPort:    0,
			})))
		})

		Context("there is no owner for a pod", func() {

			BeforeEach(func() {
				pod0.OwnerReferences = []meta.OwnerReference{}
			})

			It("should not send routes for the pod", func() {
				Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(Equal(&route.Message{
					Name:       "mr-stateful-0",
					Routes:     []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       8080,
					TLSPort:    0,
				})))
			})

			It("should not prevent other routes to be sent", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
					Name:       "mr-stateful-1",
					Routes:     []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       8080,
					TLSPort:    0,
				})))
			})

			It("should log the error", func() {
				Eventually(logger.LogMessages, routeMessageTimeout).Should(ContainElement("test.unexpected-pod-owner"))
			})
		})
	})

	Context("When a pod is deleted", func() {

		BeforeEach(func() {
			pod0 = createPod("mr-stateful-0")
			pod0.Status = v1.PodStatus{PodIP: "10.20.30.40"}
			pod1 = createPod("mr-stateful-1")
			pod1.Status = v1.PodStatus{PodIP: "50.60.70.80"}
		})

		JustBeforeEach(func() {
			podWatcher.Delete(pod0)
		})

		It("should send the unregister routes", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Name:               "mr-stateful-0",
				UnregisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
				InstanceID:         "mr-stateful-0",
				Address:            "10.20.30.40",
				Port:               8080,
				TLSPort:            0,
			})))
		})

		It("should NOT send a unregister message for other pods", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(Equal(&route.Message{
				Name:               "mr-stateful-1",
				UnregisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
				InstanceID:         "mr-stateful-1",
				Address:            "50.60.70.80",
				Port:               8080,
				TLSPort:            0,
			})))
		})

		Context("there is no owner for a pod", func() {

			BeforeEach(func() {
				pod0.OwnerReferences = []meta.OwnerReference{}
			})

			JustBeforeEach(func() {
				podWatcher.Delete(pod1)
			})

			It("should not send routes for the pod", func() {
				Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(Equal(&route.Message{
					Name:               "mr-stateful-0",
					UnregisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
					InstanceID:         "mr-stateful-0",
					Address:            "10.20.30.40",
					Port:               8080,
					TLSPort:            0,
				})))
			})

			It("should not prevent other routes to be sent", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
					Name:               "mr-stateful-1",
					UnregisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io", "mr-bombastic.50.60.70.80.nip.io"},
					InstanceID:         "mr-stateful-1",
					Address:            "50.60.70.80",
					Port:               8080,
					TLSPort:            0,
				})))
			})

			It("should log the error", func() {
				Eventually(logger.LogMessages, routeMessageTimeout).Should(ContainElement("test.unexpected-pod-owner"))
			})
		})
	})
})
