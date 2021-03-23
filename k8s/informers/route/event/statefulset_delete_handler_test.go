package event_test

import (
	"time"

	"code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/k8s/informers/route/event"
	"code.cloudfoundry.org/eirini/k8s/informers/route/event/eventfakes"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	eiriniroutefakes "code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatefulsetDeleteHandler", func() {
	var (
		handler            route.StatefulSetDeleteEventHandler
		podClient          *eventfakes.FakePodInterface
		routeEmitter       *eiriniroutefakes.FakeEmitter
		logger             *lagertest.TestLogger
		deletedStatefulSet *appsv1.StatefulSet
	)

	BeforeEach(func() {
		podClient = new(eventfakes.FakePodInterface)
		routeEmitter = new(eiriniroutefakes.FakeEmitter)
		logger = lagertest.NewTestLogger("uri-informer-test")

		handler = event.StatefulSetDeleteHandler{
			Pods:         podClient,
			Logger:       logger,
			RouteEmitter: routeEmitter,
		}

		deletedStatefulSet = createStatefulSetWithRoutes(`[
						{
							"hostname": "mr-stateful.cf.domain",
							"port": 8080
						},
						{
							"hostname": "mr-boombastic.cf.domain",
							"port": 6565
						}
			]`)
	})

	assertUnregisteredRoutesForAllPods := func() {
		allArgs := []eiriniroute.Message{}
		for i := 0; i < routeEmitter.EmitCallCount(); i++ {
			_, r := routeEmitter.EmitArgsForCall(i)
			allArgs = append(allArgs, r)
		}
		Expect(routeEmitter.EmitCallCount()).To(Equal(4))

		Expect(allArgs).To(ContainElement(eiriniroute.Message{
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
		Expect(allArgs).To(ContainElement(eiriniroute.Message{
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
		Expect(allArgs).To(ContainElement(eiriniroute.Message{
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
		Expect(allArgs).To(ContainElement(eiriniroute.Message{
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
	}

	Context("When the app is deleted", func() {
		BeforeEach(func() {
			podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
				createPod("mr-stateful-0", "10.20.30.40"),
				createPod("mr-stateful-1", "50.60.70.80"),
			}}, nil)
		})

		It("should unregister all routes for all pods", func() {
			handler.Handle(ctx, deletedStatefulSet)
			assertUnregisteredRoutesForAllPods()
		})

		Context("and a pod is not ready", func() {
			BeforeEach(func() {
				pod := createPod("mr-stateful-0", "10.20.30.40")
				pod.Status.Conditions[0].Status = corev1.ConditionFalse

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					pod,
					createPod("mr-stateful-1", "50.60.70.80"),
				}}, nil)
			})

			It("should unregister all routes for all pods", func() {
				handler.Handle(ctx, deletedStatefulSet)
				assertUnregisteredRoutesForAllPods()
			})
		})

		Context("and the pod is marked for deletion", func() {
			BeforeEach(func() {
				pod := createPod("mr-stateful-0", "10.20.30.40")
				pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				podClient.ListReturns(&corev1.PodList{Items: []corev1.Pod{
					pod,
					createPod("mr-stateful-1", "50.60.70.80"),
				}}, nil)
			})

			It("should unregister all routes for all pods", func() {
				handler.Handle(ctx, deletedStatefulSet)
				assertUnregisteredRoutesForAllPods()
			})
		})

		Context("and decoding routes fails", func() {
			BeforeEach(func() {
				handler.Handle(ctx, createStatefulSetWithRoutes(`[`))
			})

			It("shouldn't send any messages", func() {
				Expect(routeEmitter.EmitCallCount()).To(BeZero())
			})

			It("should provide a helpful message", func() {
				Expect(logger.Logs()).NotTo(BeEmpty())

				log := logger.Logs()[0]
				Expect(log.Message).To(Equal("uri-informer-test.statefulset-delete.failed-to-decode-deleted-user-defined-routes"))
				Expect(log.Data).To(HaveKeyWithValue("guid", "myguid"))
				Expect(log.LogLevel).To(Equal(lager.ERROR))
				Expect(log.Data).To(HaveKeyWithValue("error", "failed to unmarshal routes: unexpected end of JSON input"))
			})
		})
	})
})
