package event_test

import (
	"time"

	"code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/k8s/informers/route/event"
	"code.cloudfoundry.org/eirini/k8s/informers/route/event/eventfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	eiriniroutefakes "code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatefulsetAnnotationEventHandler", func() {
	var (
		handler            route.StatefulSetUpdateEventHandler
		podClient          *eventfakes.FakePodInterface
		routeEmitter       *eiriniroutefakes.FakeEmitter
		logger             *lagertest.TestLogger
		oldStatefulSet     *appsv1.StatefulSet
		updatedStatefulSet *appsv1.StatefulSet
		allEmitArgs        []eiriniroute.Message
	)

	BeforeEach(func() {
		podClient = new(eventfakes.FakePodInterface)
		routeEmitter = new(eiriniroutefakes.FakeEmitter)
		logger = lagertest.NewTestLogger("uri-informer-test")
		allEmitArgs = []eiriniroute.Message{}

		handler = event.URIAnnotationUpdateHandler{
			Pods:         podClient,
			Logger:       logger,
			RouteEmitter: routeEmitter,
		}
	})

	Context("When a new route is added by the user", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-fantastic.cf.domain",
							"port": 7563
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)
		})

		It("should get all children pods of the statefulset", func() {
			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			Expect(podClient.ListCallCount()).To(Equal(1))
			_, listOptions := podClient.ListArgsForCall(0)
			Expect(listOptions.LabelSelector).To(Equal("name=the-app-name"))
		})

		It("should emit all routes for each pod", func() {
			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			Expect(routeEmitter.EmitCallCount()).To(Equal(6))

			allArgs := []eiriniroute.Message{}
			for i := 0; i < routeEmitter.EmitCallCount(); i++ {
				_, r := routeEmitter.EmitArgsForCall(i)
				allArgs = append(allArgs, r)
			}

			Expect(allArgs).To(ConsistOf(eiriniroute.Message{
				Name: "mr-stateful-0-guid",
				Routes: eiriniroute.Routes{
					RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
					UnregisteredRoutes: nil,
				},
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       8080,
				TLSPort:    0,
			},
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-fantastic.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       7563,
					TLSPort:    0,
				},
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-boombastic.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       6565,
					TLSPort:    0,
				},
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       8080,
					TLSPort:    0,
				},
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-fantastic.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       7563,
					TLSPort:    0,
				},
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-boombastic.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       6565,
					TLSPort:    0,
				},
			))
		})

		Context("and a pod is marked for deletion", func() {
			It("should not send routes for the pod", func() {
				now := metav1.Time{Time: time.Now()}
				pod0 := createPod("mr-stateful-0", "10.20.30.40")
				pod0.DeletionTimestamp = &now

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					pod0,
					createPod("mr-stateful-1", "50.60.70.80"),
				}}, nil)

				handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

				Expect(routeEmitter.EmitCallCount()).To(Equal(3))

				allArgs := []eiriniroute.Message{}
				for i := 0; i < routeEmitter.EmitCallCount(); i++ {
					_, r := routeEmitter.EmitArgsForCall(i)
					allArgs = append(allArgs, r)
				}

				Expect(allArgs).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
					"InstanceID": Equal("mr-stateful-0"),
				})))
			})
		})

		Context("and a pod is not ready", func() {
			It("should not send routes for the pod", func() {
				pod1 := createPod("mr-stateful-1", "50.60.70.80")
				pod1.Status.Conditions[0].Status = corev1.ConditionFalse

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					createPod("mr-stateful-0", "10.20.30.40"),
					pod1,
				}}, nil)

				handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

				Expect(routeEmitter.EmitCallCount()).To(Equal(3))

				allArgs := []eiriniroute.Message{}
				for i := 0; i < routeEmitter.EmitCallCount(); i++ {
					_, r := routeEmitter.EmitArgsForCall(i)
					allArgs = append(allArgs, r)
				}

				Expect(allArgs).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
					"InstanceID": Equal("mr-stateful-1"),
				})))
			})
		})

		Context("and a pod has no ip", func() {
			It("should not send routes for the pod", func() {
				pod1 := createPod("mr-stateful-1", "50.60.70.80")
				pod1.Status.PodIP = ""

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					createPod("mr-stateful-0", "10.20.30.40"),
					pod1,
				}}, nil)

				handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

				Expect(routeEmitter.EmitCallCount()).To(Equal(3))

				allArgs := []eiriniroute.Message{}
				for i := 0; i < routeEmitter.EmitCallCount(); i++ {
					_, r := routeEmitter.EmitArgsForCall(i)
					allArgs = append(allArgs, r)
				}

				Expect(allArgs).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
					"InstanceID": Equal("mr-stateful-1"),
				})))
			})

			It("should register routes for the other pods", func() {
				pod1 := createPod("mr-stateful-1", "50.60.70.80")
				pod1.Status.PodIP = ""

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					createPod("mr-stateful-0", "10.20.30.40"),
					pod1,
				}}, nil)

				handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

				Expect(routeEmitter.EmitCallCount()).To(Equal(3))
				allArgs := []eiriniroute.Message{}
				for i := 0; i < routeEmitter.EmitCallCount(); i++ {
					_, r := routeEmitter.EmitArgsForCall(i)
					allArgs = append(allArgs, r)
				}

				Expect(allArgs).To(ConsistOf(eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       8080,
					TLSPort:    0,
				},
					eiriniroute.Message{
						Name: "mr-stateful-0-guid",
						Routes: eiriniroute.Routes{
							RegisteredRoutes:   []string{"mr-fantastic.cf.domain"},
							UnregisteredRoutes: nil,
						},
						InstanceID: "mr-stateful-0",
						Address:    "10.20.30.40",
						Port:       7563,
						TLSPort:    0,
					},
					eiriniroute.Message{
						Name: "mr-stateful-0-guid",
						Routes: eiriniroute.Routes{
							RegisteredRoutes:   []string{"mr-boombastic.cf.domain"},
							UnregisteredRoutes: nil,
						},
						InstanceID: "mr-stateful-0",
						Address:    "10.20.30.40",
						Port:       6565,
						TLSPort:    0,
					},
				))
			})

			It("should provide a helpful log message", func() {
				pod1 := createPod("mr-stateful-1", "50.60.70.80")
				pod1.Status.PodIP = ""

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					createPod("mr-stateful-0", "10.20.30.40"),
					pod1,
				}}, nil)

				handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

				Expect(logger.Logs()).NotTo(BeEmpty())

				log := logger.Logs()[0]
				Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-construct-a-route-message"))
				Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
				Expect(log.Data).To(HaveKeyWithValue("error", "missing ip address"))
			})
		})
	})

	Context("When a route is removed by the user", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						}
					]`)

			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			for i := 0; i < routeEmitter.EmitCallCount(); i++ {
				_, r := routeEmitter.EmitArgsForCall(i)
				allEmitArgs = append(allEmitArgs, r)
			}
		})

		It("should unregister the deleted route for the first pod", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   nil,
						UnregisteredRoutes: []string{"mr-boombastic.cf.domain"},
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       6565,
					TLSPort:    0,
				},
			))
		})

		It("should unregister the deleted route for the second pod", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   nil,
						UnregisteredRoutes: []string{"mr-boombastic.cf.domain"},
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       6565,
					TLSPort:    0,
				},
			))
		})
	})

	Context("when the port of a route is changed", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 1111
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)

			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			for i := 0; i < routeEmitter.EmitCallCount(); i++ {
				_, r := routeEmitter.EmitArgsForCall(i)
				allEmitArgs = append(allEmitArgs, r)
			}
		})

		It("should register the new port for all pods", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       1111,
					TLSPort:    0,
				},
			))

			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       1111,
					TLSPort:    0,
				},
			))
		})

		It("should unregister the new port for all pods", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   nil,
						UnregisteredRoutes: []string{"mr-stateful.cf.domain"},
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       8080,
					TLSPort:    0,
				},
			))

			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   nil,
						UnregisteredRoutes: []string{"mr-stateful.cf.domain"},
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       8080,
					TLSPort:    0,
				},
			))
		})
	})

	Context("When a route shares a port with another route", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 8080
						}
					]`)

			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			for i := 0; i < routeEmitter.EmitCallCount(); i++ {
				_, r := routeEmitter.EmitArgsForCall(i)
				allEmitArgs = append(allEmitArgs, r)
			}
		})

		It("should register both routes in a single message", func() {
			Expect(allEmitArgs).To(ConsistOf(
				MatchAllFields(Fields{
					"Name": Equal("mr-stateful-0-guid"),
					"Routes": MatchAllFields(Fields{
						"RegisteredRoutes":   ConsistOf("mr-stateful.cf.domain", "mr-boombastic.cf.domain"),
						"UnregisteredRoutes": BeEmpty(),
					}),
					"InstanceID": Equal("mr-stateful-0"),
					"Address":    Equal("10.20.30.40"),
					"Port":       BeNumerically("==", 8080),
					"TLSPort":    BeNumerically("==", 0),
				}),
				MatchAllFields(Fields{
					"Name": Equal("mr-stateful-1-guid"),
					"Routes": MatchAllFields(Fields{
						"RegisteredRoutes":   ConsistOf("mr-stateful.cf.domain", "mr-boombastic.cf.domain"),
						"UnregisteredRoutes": BeEmpty(),
					}),
					"InstanceID": Equal("mr-stateful-1"),
					"Address":    Equal("50.60.70.80"),
					"Port":       BeNumerically("==", 8080),
					"TLSPort":    BeNumerically("==", 0),
				}),
			))
		})
	})

	Context("When decoding updated user defined routes fails", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[`)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)
		})

		It("should not register a new route", func() {
			Expect(routeEmitter.EmitCallCount()).To(Equal(0))
		})

		It("should provide a helpful message", func() {
			Expect(logger.Logs()).ToNot(BeEmpty())

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-decode-updated-user-defined-routes"))
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
			Expect(log.Data).To(HaveKeyWithValue("error", "failed to unmarshal routes: unexpected end of JSON input"))
		})
	})

	Context("When decoding old user defined routes fails", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						}
				]`)

			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			for i := 0; i < routeEmitter.EmitCallCount(); i++ {
				_, r := routeEmitter.EmitArgsForCall(i)
				allEmitArgs = append(allEmitArgs, r)
			}
		})

		It("should still register the new route", func() {
			Expect(allEmitArgs).To(ConsistOf(
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       8080,
					TLSPort:    0,
				},
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       8080,
					TLSPort:    0,
				},
			))
		})

		It("should provide a helpful message", func() {
			Expect(logger.Logs()).NotTo(BeEmpty())

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-decode-old-user-defined-routes"))
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
			Expect(log.Data).To(HaveKeyWithValue("error", "failed to unmarshal routes: unexpected end of JSON input"))
		})
	})

	Context("When the pods cannot be listed", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)

			podClient.ListReturns(nil, errors.New("listing pods went boom"))

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)
		})

		It("should not send any routes", func() {
			Expect(routeEmitter.EmitCallCount()).To(Equal(0))
		})

		It("should provide a helpful log message", func() {
			Expect(logger.Logs()).NotTo(BeEmpty())
			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("uri-informer-test.statefulset-update.failed-to-get-child-pods"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Data).To(HaveKeyWithValue("error", "failed to list pods: listing pods went boom"))
		})
	})

	Context("When a pod is not ready", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 1111
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)

			pod0 := createPod("mr-stateful-0", "10.20.30.40")
			pod0.Status.Conditions[0].Status = corev1.ConditionFalse
			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				pod0,
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)

			for i := 0; i < routeEmitter.EmitCallCount(); i++ {
				_, r := routeEmitter.EmitArgsForCall(i)
				allEmitArgs = append(allEmitArgs, r)
			}
		})

		It("should not send routes for the pod", func() {
			Expect(allEmitArgs).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name":             Equal("mr-stateful-0-guid"),
				"RegisteredRoutes": Not(BeEmpty()),
			})))
		})

		It("should unregister the deleted route for the pod", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-0-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   nil,
						UnregisteredRoutes: []string{"mr-stateful.cf.domain"},
					},
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       8080,
					TLSPort:    0,
				}))
		})

		It("should register the new route for the other pod", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   []string{"mr-stateful.cf.domain"},
						UnregisteredRoutes: nil,
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       1111,
					TLSPort:    0,
				}))
		})

		It("should unregister the deleted route for the other pod", func() {
			Expect(allEmitArgs).To(ContainElement(
				eiriniroute.Message{
					Name: "mr-stateful-1-guid",
					Routes: eiriniroute.Routes{
						RegisteredRoutes:   nil,
						UnregisteredRoutes: []string{"mr-stateful.cf.domain"},
					},
					InstanceID: "mr-stateful-1",
					Address:    "50.60.70.80",
					Port:       8080,
					TLSPort:    0,
				}))
		})
	})

	Context("When the annotations are not updated", func() {
		BeforeEach(func() {
			oldStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)
			updatedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
					]`)

			updatedStatefulSet.Labels = map[string]string{"new": "label"}

			handler.Handle(ctx, oldStatefulSet, updatedStatefulSet)
		})

		It("should do nothing", func() {
			Expect(routeEmitter.EmitCallCount()).To(BeZero())

			logCount := len(logger.Logs())
			Expect(logCount).To(BeZero())
		})
	})
})

func createStatefulSetWithRoutes(routes string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mr-stateful",
			Annotations: map[string]string{
				stset.AnnotationRegisteredRoutes: routes,
				stset.AnnotationProcessGUID:      "myguid",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "the-app-name",
				},
			},
		},
	}
}

func createPod(name, ip string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "StatefulSet",
					Name: "mr-stateful",
				},
			},
			Labels: map[string]string{
				stset.LabelGUID: name + "-guid",
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
