package route_test

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"

	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("InstanceChangeInformer", func() {

	const (
		namespace           = "test-me"
		routeMessageTimeout = 30 * time.Millisecond
	)

	var (
		informer   eiriniroute.Informer
		client     kubernetes.Interface
		podWatcher *watch.FakeWatcher
		workChan   chan *eiriniroute.Message
		stopChan   chan struct{}
		logger     *lagertest.TestLogger
	)

	setWatcher := func(cs kubernetes.Interface) {
		fakecs := cs.(*fake.Clientset)
		podWatcher = watch.NewFake()
		fakecs.PrependWatchReactor("pods", testcore.DefaultWatchReactor(podWatcher, nil))
	}

	createPod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"guid": name + "-guid",
				},
				Annotations: map[string]string{cf.ProcessGUID: name + "-anno"},
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "StatefulSet",
						Name: "mr-stateful",
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Ports: []corev1.ContainerPort{{ContainerPort: 8080}}},
				},
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
	}

	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		setWatcher(client)

		stopChan = make(chan struct{})
		workChan = make(chan *eiriniroute.Message, 5)

		logger = lagertest.NewTestLogger("instance-informer-test")

		informer = &InstanceChangeInformer{
			Client:    client,
			Cancel:    stopChan,
			Namespace: namespace,
			Logger:    logger,
		}
	})

	AfterEach(func() {
		close(stopChan)
	})

	JustBeforeEach(func() {
		go informer.Start(workChan)

		st := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mr-stateful",
				Annotations: map[string]string{
					eirini.RegisteredRoutes: `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 8080
						},
						{
							"hostname": "mr-bombastic.50.60.70.80.nip.io",
							"port": 6565
						}
					]`,
				},
			},
		}
		_, err := client.AppsV1().StatefulSets(namespace).Create(st)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When a updated pod is missing its IP", func() {
		It("should not send a route for the pod", func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)

			pod0.Status.Message = "where my IP at?"
			podWatcher.Modify(pod0)

			Consistently(workChan, routeMessageTimeout*2).ShouldNot(Receive())
		})

		It("should provide a helpful error message", func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)
			pod0.Status.Message = "where my IP at?"
			podWatcher.Modify(pod0)

			Eventually(func() int {
				logs := logger.Logs()
				return len(logs)
			}).Should(BeNumerically(">", 0))

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("instance-informer-test.pod-update.failed-to-construct-a-route-message"))
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Data).To(HaveKeyWithValue("pod-name", "mr-stateful-0"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "mr-stateful-0-anno"))
			Expect(log.Data).To(HaveKeyWithValue("error", "missing ip address"))
		})
	})

	Context("When an ip is assigned to a pod", func() {

		It("should send the first route", func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)
			pod0.Status.PodIP = "10.20.30.40"
			podWatcher.Modify(pod0)

			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Routes: route.Routes{
					RegisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       8080,
				TLSPort:    0,
			})))
		})

		It("should send the second route", func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)
			pod0.Status.PodIP = "10.20.30.40"
			podWatcher.Modify(pod0)

			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Routes: route.Routes{
					RegisteredRoutes: []string{"mr-bombastic.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       6565,
				TLSPort:    0,
			})))
		})

	})

	Context("When there is no owner for a pod", func() {
		JustBeforeEach(func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)
			pod0.Status.PodIP = "10.20.30.40"
			pod0.OwnerReferences = []metav1.OwnerReference{}
			podWatcher.Modify(pod0)
		})

		It("should not send routes for the pod", func() {
			Consistently(workChan, routeMessageTimeout*2).ShouldNot(Receive())
		})

		It("should provide a helpful error message", func() {
			Eventually(func() int {
				logs := logger.Logs()
				return len(logs)
			}).Should(BeNumerically(">", 0))

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("instance-informer-test.pod-update.failed-to-get-user-defined-routes"))
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Data).To(HaveKeyWithValue("pod-name", "mr-stateful-0"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "mr-stateful-0-anno"))
			Expect(log.Data).To(HaveKeyWithValue("error", ContainSubstring("there are no owners")))
		})
	})

	Context("When there are multiple pod owners", func() {

		It("should find the StatefulSet owner and send the route", func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)
			pod0.Status.PodIP = "10.20.30.40"
			pod0.OwnerReferences = append(pod0.OwnerReferences, metav1.OwnerReference{
				Kind: "extraterrestrial",
				Name: "E.T.",
			})
			podWatcher.Modify(pod0)

			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Routes: route.Routes{
					RegisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       8080,
				TLSPort:    0,
			})))
		})

		It("should not send the route if there's no StatefulSet owner", func() {
			pod0 := createPod("mr-stateful-0")
			podWatcher.Add(pod0)
			pod0.Status.PodIP = "10.20.30.40"
			pod0.OwnerReferences = []metav1.OwnerReference{
				{
					Kind: "extraterrestrial",
					Name: "E.T.",
				},
			}
			podWatcher.Modify(pod0)

			Consistently(workChan, routeMessageTimeout*2).ShouldNot(Receive())
			Eventually(func() int {
				logs := logger.Logs()
				return len(logs)
			}).Should(BeNumerically(">", 0))

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("instance-informer-test.pod-update.failed-to-get-user-defined-routes"))
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Data).To(HaveKeyWithValue("pod-name", "mr-stateful-0"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "mr-stateful-0-anno"))
			Expect(log.Data).To(HaveKeyWithValue("error", ContainSubstring("there are no statefulset owners")))
		})

	})
	Context("When a pod is marked for deletion", func() {
		var deletionTimestamp metav1.Time

		JustBeforeEach(func() {
			pod0 := createPod("mr-stateful-0")
			pod0.Status.PodIP = "10.20.30.40"
			podWatcher.Add(pod0)
			deletionTimestamp = metav1.Time{
				Time: time.Now(),
			}
			pod0.DeletionTimestamp = &deletionTimestamp
			podWatcher.Modify(pod0)
		})

		It("sends unregister route message for the pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
				Routes: route.Routes{
					UnregisteredRoutes: []string{"mr-bombastic.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       6565,
				TLSPort:    0,
			})))
		})

		It("should provide a helpful error message", func() {
			Eventually(logger.Logs).ShouldNot(BeEmpty())

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("instance-informer-test.pod-update.pod-not-ready"))
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Data).To(HaveKeyWithValue("pod-name", "mr-stateful-0"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "mr-stateful-0-anno"))
			Expect(log.Data).To(HaveKeyWithValue("statuses", HaveLen(1)))
			Expect(log.Data).To(HaveKeyWithValue("deletion-timestamp", deletionTimestamp.UTC().Format("2006-01-02T15:04:05Z")))

			bytes, err := json.Marshal(log.Data["statuses"])
			Expect(err).ToNot(HaveOccurred())
			var conditions []corev1.PodCondition
			err = json.Unmarshal(bytes, &conditions)
			Expect(err).ToNot(HaveOccurred())
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Status).To(Equal(corev1.ConditionTrue))
		})

	})

	Context("When pod is not ready", func() {
		Context("when pod was ready before", func() {
			JustBeforeEach(func() {
				pod0 := createPod("mr-stateful-0")
				pod0.Status.PodIP = "10.20.30.40"
				podWatcher.Add(pod0)
				updatedPod0 := createPod("mr-stateful-0")
				updatedPod0.Status.Conditions[0].Status = corev1.ConditionFalse
				podWatcher.Modify(updatedPod0)
			})

			It("sends unregister route message for the pod", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(Equal(&route.Message{
					Routes: route.Routes{
						UnregisteredRoutes: []string{"mr-bombastic.50.60.70.80.nip.io"},
					},

					Name:       "mr-stateful-0-guid",
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       6565,
					TLSPort:    0,
				})))
			})

			It("should provide a helpful error message", func() {
				Eventually(func() int {
					logs := logger.Logs()
					return len(logs)
				}).Should(BeNumerically(">", 0))

				log := logger.Logs()[0]
				Expect(log.Message).To(Equal("instance-informer-test.pod-update.pod-not-ready"))
				Expect(log.LogLevel).To(Equal(lager.DEBUG))
				Expect(log.Data).To(HaveKeyWithValue("pod-name", "mr-stateful-0"))
				Expect(log.Data).To(HaveKeyWithValue("guid", "mr-stateful-0-anno"))
				Expect(log.Data).To(HaveKeyWithValue("statuses", HaveLen(1)))
				Expect(log.Data).To(HaveKeyWithValue("deletion-timestamp", BeNil()))

				bytes, err := json.Marshal(log.Data["statuses"])
				Expect(err).ToNot(HaveOccurred())
				var conditions []corev1.PodCondition
				err = json.Unmarshal(bytes, &conditions)
				Expect(err).ToNot(HaveOccurred())
				Expect(conditions).To(HaveLen(1))
				Expect(conditions[0].Status).To(Equal(corev1.ConditionFalse))
			})
		})

		Context("pod ready condition is missing", func() {
			It("should not send routes for the pod", func() {
				pod0 := createPod("mr-stateful-0")
				podWatcher.Add(pod0)
				pod0.Status.Conditions[0].Type = corev1.PodInitialized
				podWatcher.Modify(pod0)

				Consistently(workChan, routeMessageTimeout*2).ShouldNot(Receive())
			})
		})

		Context("and the old pod readiness status is false", func() {
			It("should not send routes for the pod", func() {
				pod0 := createPod("mr-stateful-0")
				podWatcher.Add(pod0)
				pod0.Status.Conditions[0].Status = corev1.ConditionFalse
				podWatcher.Modify(pod0)

				Consistently(workChan, routeMessageTimeout*2).ShouldNot(Receive())
			})
		})
	})
})
