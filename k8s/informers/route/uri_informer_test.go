package route_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/models/cf"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
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
		workChan    chan *eiriniroute.Message
		stopChan    chan struct{}
		logger      *lagertest.TestLogger
		statefulset *appsv1.StatefulSet
		pod0, pod1  *corev1.Pod
		informerWG  sync.WaitGroup
	)

	createPod := func(name, ip string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "StatefulSet",
						Name: "mr-stateful",
					},
				},
				Labels: map[string]string{
					"name": "the-app-name",
					"guid": name + "-guid",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Ports: []corev1.ContainerPort{{ContainerPort: 8080}}},
				},
			},
			Status: corev1.PodStatus{
				PodIP: ip,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
	}

	setWatcher := func(cs kubernetes.Interface) {
		fakecs := cs.(*fake.Clientset)
		watcher = watch.NewFake()
		fakecs.PrependWatchReactor("statefulsets", testcore.DefaultWatchReactor(watcher, nil))
	}

	copyWithModifiedRoute := func(st *appsv1.StatefulSet, routes string) *appsv1.StatefulSet {
		thecopy := *st

		thecopy.Annotations = map[string]string{
			eirini.RegisteredRoutes: routes,
		}
		return &thecopy
	}

	BeforeEach(func() {
		pod0 = createPod("mr-stateful-0", "10.20.30.40")
		pod1 = createPod("mr-stateful-1", "50.60.70.80")

		client = fake.NewSimpleClientset()
		setWatcher(client)

		stopChan = make(chan struct{})
		workChan = make(chan *eiriniroute.Message, 5)

		logger = lagertest.NewTestLogger("uri-informer-test")

		statefulset = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mr-stateful",
				Annotations: map[string]string{
					eirini.RegisteredRoutes: `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.50.60.70.80.nip.io",
							"port": 6565
						}
					]`,
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": "the-app-name",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							cf.ProcessGUID: "myguid",
						},
					},
				},
			},
		}

		informer = URIChangeInformer{
			Client:    client,
			Cancel:    stopChan,
			Namespace: namespace,
			Logger:    logger,
		}
	})

	AfterEach(func() {
		close(stopChan)
		informerWG.Wait()
		close(workChan)
	})

	JustBeforeEach(func() {
		informerWG.Add(1)
		go func() {
			informer.Start(workChan)
			informerWG.Done()
		}()

		watcher.Add(statefulset)

		_, err := client.CoreV1().Pods(namespace).Create(pod0)
		Expect(err).ToNot(HaveOccurred())
		_, err = client.CoreV1().Pods(namespace).Create(pod1)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When a new route is added by the user", func() {
		JustBeforeEach(func() {
			newRoutes := `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 8080
						},
						{
							"hostname": "mr-fantastic.50.60.70.80.nip.io",
							"port": 7563
						},
						{
							"hostname": "mr-boombastic.50.60.70.80.nip.io",
							"port": 6565
						}
					]`
			watcher.Modify(copyWithModifiedRoute(statefulset, newRoutes))
		})

		It("should register the first new route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should register the second new route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-fantastic.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 7563),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})
		It("should register the third new route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 6565),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should register the first new route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should register the second new route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-fantastic.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 7563),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should register the third new route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 6565),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		Context("and the pod is marked for deletion", func() {
			BeforeEach(func() {
				now := metav1.Time{
					Time: time.Now(),
				}

				pod0.DeletionTimestamp = &now
			})

			It("should not send routes for the pod", func() {
				Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal("mr-stateful-0"),
				}))))
			})
		})

		Context("and the first pod is not ready", func() {
			BeforeEach(func() {
				pod0.Status.Conditions[0].Status = corev1.ConditionFalse
			})

			It("should not send routes for the pod", func() {
				Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal("mr-stateful-0-guid"),
				}))))
			})
		})

		Context("and the first pod has no ip", func() {
			BeforeEach(func() {
				pod0.Status.PodIP = ""
			})

			It("should not send routes for the first pod", func() {
				Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal("mr-stateful-0-guid"),
				}))))
			})

			It("should register routes for the second pod", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal("mr-stateful-1-guid"),
					"Routes": MatchFields(IgnoreExtras, Fields{
						"RegisteredRoutes": Not(BeEmpty()),
					}),
				}))))
			})

			It("should provide a helpful log message", func() {
				Eventually(func() int {
					logs := logger.Logs()
					return len(logs)
				}).Should(BeNumerically(">", 0))

				log := logger.Logs()[0]
				Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-construct-a-route-message"))
				Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
				Expect(log.Data).To(HaveKeyWithValue("error", "missing ip address"))
			})
		})
	})

	Context("When a route is removed by the user", func() {

		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `[
			{
				"hostname": "mr-stateful.50.60.70.80.nip.io",
				"port": 8080
			}]`))
		})

		It("should unregister the deleted route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 6565),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister the deleted route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 6565),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})
	})

	Context("when the port of a route is changed", func() {
		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 1111
						},
						{
							"hostname": "mr-boombastic.50.60.70.80.nip.io",
							"port": 6565
						}
					]`))
		})

		It("should unregister the first changed route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 1111),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister the firt changed route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister the first changed route for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 1111),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister the first changed route for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})
	})

	Context("When a route shares a port with another route", func() {

		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.50.60.70.80.nip.io",
							"port": 8080
						}
					]`))
		})

		It("should register both routes in a single message", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io", "mr-boombastic.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should register both routes in a single message", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io", "mr-boombastic.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})
	})

	Context("When decoding updated user defined routes fails", func() {
		BeforeEach(func() {
			statefulset.Annotations[eirini.RegisteredRoutes] = "[]"
		})

		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `[`))
		})

		It("should not register a new route", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive())
		})

		It("should provide a helpful message", func() {
			Eventually(func() int {
				logs := logger.Logs()
				return len(logs)
			}).Should(BeNumerically(">", 0))

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-decode-updated-user-defined-routes"))
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
			Expect(log.Data).To(HaveKeyWithValue("error", "unexpected end of JSON input"))
		})
	})

	Context("When decoding old user defined routes fails", func() {
		BeforeEach(func() {
			statefulset.Annotations[eirini.RegisteredRoutes] = "["
		})

		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 8080
						}
					]`))
		})

		It("should still register the new route", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should provide a helpful message", func() {
			Eventually(func() int {
				logs := logger.Logs()
				return len(logs)
			}).Should(BeNumerically(">", 0))

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-decode-old-user-defined-routes"))
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
			Expect(log.Data).To(HaveKeyWithValue("error", "unexpected end of JSON input"))
		})

	})

	Context("When the pods cannot be listed", func() {

		BeforeEach(func() {
			reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("listing pods went boom")
			}
			informer.Client.(*fake.Clientset).PrependReactor("list", "pods", reaction)
		})

		JustBeforeEach(func() {
			newRoutes := `[
			{
				"hostname": "shaggy.50.60.70.80.nip.io",
				"port": 4545
			}]`
			watcher.Modify(copyWithModifiedRoute(statefulset, newRoutes))
		})

		It("should not send any routes", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive())
		})

		It("should provide a helpful log message", func() {
			Eventually(func() int {
				logs := logger.Logs()
				return len(logs)
			}).Should(BeNumerically(">", 0))

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-get-child-pods"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Data).To(HaveKeyWithValue("error", "listing pods went boom"))
		})
	})

	Context("When a pod is not ready", func() {

		BeforeEach(func() {
			pod0.Status.Conditions[0].Status = corev1.ConditionFalse
		})

		JustBeforeEach(func() {
			watcher.Modify(copyWithModifiedRoute(statefulset, `[
						{
							"hostname": "mr-stateful.50.60.70.80.nip.io",
							"port": 1111
						},
						{
							"hostname": "mr-boombastic.50.60.70.80.nip.io",
							"port": 6565
						}
					]`))
		})

		It("should not send routes for the pod", func() {
			Consistently(workChan, routeMessageTimeout).ShouldNot(Receive(PointTo(MatchFields(IgnoreExtras, Fields{
				"Name":             Equal("mr-stateful-0-guid"),
				"RegisteredRoutes": Not(BeEmpty()),
			}))))
		})

		It("should unregister the deleted route for the pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should register the new route for the other pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					"UnregisteredRoutes": BeEmpty(),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 1111),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister the deleted route for the other pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

	})

	Context("When the annotations are not updated", func() {

		JustBeforeEach(func() {
			updated := statefulset.DeepCopy()
			updated.Labels = map[string]string{"new": "label"}
			watcher.Modify(updated)
		})

		It("should do nothing", func() {
			Consistently(workChan).ShouldNot(Receive())

			logCount := len(logger.Logs())
			Expect(logCount).To(BeZero())
		})
	})

	Context("When the app is deleted", func() {

		JustBeforeEach(func() {
			watcher.Delete(statefulset)
		})

		It("should unregister all routes for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister all routes for the first pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-0-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-0"),
				"Address":    Equal("10.20.30.40"),
				"Port":       BeNumerically("==", 6565),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister all routes for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 8080),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		It("should unregister all routes for the second pod", func() {
			Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
				"Name": Equal("mr-stateful-1-guid"),
				"Routes": MatchAllFields(Fields{
					"RegisteredRoutes":   BeEmpty(),
					"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
				}),
				"InstanceID": Equal("mr-stateful-1"),
				"Address":    Equal("50.60.70.80"),
				"Port":       BeNumerically("==", 6565),
				"TLSPort":    BeNumerically("==", 0),
			}))))
		})

		Context("and a pod is not ready", func() {
			BeforeEach(func() {
				pod0.Status.Conditions[0].Status = corev1.ConditionFalse
			})

			It("should unregister all routes for the first pod", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
					"Name": Equal("mr-stateful-0-guid"),
					"Routes": MatchAllFields(Fields{
						"RegisteredRoutes":   BeEmpty(),
						"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					}),
					"InstanceID": Equal("mr-stateful-0"),
					"Address":    Equal("10.20.30.40"),
					"Port":       BeNumerically("==", 8080),
					"TLSPort":    BeNumerically("==", 0),
				}))))
			})

			It("should unregister all routes for the first pod", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
					"Name": Equal("mr-stateful-0-guid"),
					"Routes": MatchAllFields(Fields{
						"RegisteredRoutes":   BeEmpty(),
						"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
					}),
					"InstanceID": Equal("mr-stateful-0"),
					"Address":    Equal("10.20.30.40"),
					"Port":       BeNumerically("==", 6565),
					"TLSPort":    BeNumerically("==", 0),
				}))))
			})
		})

		Context("and the pod is marked for deletion", func() {
			BeforeEach(func() {
				now := metav1.Time{
					Time: time.Now(),
				}

				pod0.DeletionTimestamp = &now
			})

			It("should unregister all routes for the first pod", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
					"Name": Equal("mr-stateful-0-guid"),
					"Routes": MatchAllFields(Fields{
						"RegisteredRoutes":   BeEmpty(),
						"UnregisteredRoutes": ConsistOf("mr-stateful.50.60.70.80.nip.io"),
					}),
					"InstanceID": Equal("mr-stateful-0"),
					"Address":    Equal("10.20.30.40"),
					"Port":       BeNumerically("==", 8080),
					"TLSPort":    BeNumerically("==", 0),
				}))))
			})

			It("should unregister all routes for the first pod", func() {
				Eventually(workChan, routeMessageTimeout).Should(Receive(PointTo(MatchAllFields(Fields{
					"Name": Equal("mr-stateful-0-guid"),
					"Routes": MatchAllFields(Fields{
						"RegisteredRoutes":   BeEmpty(),
						"UnregisteredRoutes": ConsistOf("mr-boombastic.50.60.70.80.nip.io"),
					}),
					"InstanceID": Equal("mr-stateful-0"),
					"Address":    Equal("10.20.30.40"),
					"Port":       BeNumerically("==", 6565),
					"TLSPort":    BeNumerically("==", 0),
				}))))
			})
		})

		Context("and decoding routes fails", func() {
			BeforeEach(func() {
				statefulset.Annotations["cloudfoundry.org/routes"] = "["
			})

			It("shouldn't send any messages", func() {
				Consistently(workChan, routeMessageTimeout).ShouldNot(Receive())
			})

			It("should provide a helpful message", func() {
				Eventually(func() int {
					logs := logger.Logs()
					return len(logs)
				}).Should(BeNumerically(">", 0))

				log := logger.Logs()[0]
				Expect(log.Message).To(Equal("uri-informer-test.statefulset-delete.failed-to-decode-deleted-user-defined-routes"))
				Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
				Expect(log.LogLevel).To(Equal(lager.ERROR))
				Expect(log.Data).To(HaveKeyWithValue("error", "unexpected end of JSON input"))
			})
		})
	})
})
