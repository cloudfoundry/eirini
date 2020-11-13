package event_test

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/k8s"
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

var _ = Describe("UpdateEventHandler", func() {
	var (
		statefulSetGetter *eventfakes.FakeStatefulSetGetter
		logger            *lagertest.TestLogger
		routeEmitter      *eiriniroutefakes.FakeEmitter
		handler           route.PodUpdateEventHandler
		pod               *corev1.Pod
		updatedPod        *corev1.Pod
	)

	createPod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					k8s.LabelGUID: fmt.Sprintf("%s-guid", name),
				},
				Annotations: map[string]string{k8s.AnnotationProcessGUID: fmt.Sprintf("%s-anno", name)},
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
				PodIP: "10.20.30.40",
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
		statefulSetGetter = new(eventfakes.FakeStatefulSetGetter)
		logger = lagertest.NewTestLogger("instance-informer-test")
		routeEmitter = new(eiriniroutefakes.FakeEmitter)

		st := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mr-stateful",
				Annotations: map[string]string{
					k8s.AnnotationRegisteredRoutes: `[
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
		statefulSetGetter.GetReturns(st, nil)
		pod = createPod("mr-stateful-0")
		updatedPod = createPod("mr-stateful-0")

		handler = event.PodUpdateHandler{
			StatefulSetGetter: statefulSetGetter,
			Logger:            logger,
			RouteEmitter:      routeEmitter,
		}
	})

	Context("When a updated pod is missing its IP", func() {
		BeforeEach(func() {
			updatedPod.Status.PodIP = ""
			updatedPod.Status.Message = "where my IP at?"
		})

		It("should not send a route for the pod", func() {
			handler.Handle(pod, updatedPod)
			Expect(routeEmitter.EmitCallCount()).To(Equal(0))
		})

		It("should provide a helpful error message", func() {
			handler.Handle(pod, updatedPod)

			Expect(logger.Logs()).ToNot(BeEmpty())

			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("instance-informer-test.pod-update.failed-to-construct-a-route-message"))
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Data).To(HaveKeyWithValue("pod-name", "mr-stateful-0"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "mr-stateful-0-anno"))
			Expect(log.Data).To(HaveKeyWithValue("error", "missing ip address"))
		})
	})

	Context("When an ip is assigned to a pod", func() {
		It("should send all routes", func() {
			handler.Handle(pod, updatedPod)

			Expect(routeEmitter.EmitCallCount()).To(Equal(2))
			Expect(routeEmitter.EmitArgsForCall(0)).To(Equal(eiriniroute.Message{
				Routes: eiriniroute.Routes{
					RegisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       8080,
				TLSPort:    0,
			}))
			Expect(routeEmitter.EmitArgsForCall(1)).To(Equal(eiriniroute.Message{
				Routes: eiriniroute.Routes{
					RegisteredRoutes: []string{"mr-bombastic.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       6565,
				TLSPort:    0,
			}))
		})
	})

	Context("When there is no owner for a pod", func() {
		It("should not send routes for the pod", func() {
			updatedPod.OwnerReferences = []metav1.OwnerReference{}

			handler.Handle(pod, updatedPod)
			Expect(routeEmitter.EmitCallCount()).To(Equal(0))
		})

		It("should provide a helpful error message", func() {
			updatedPod.OwnerReferences = []metav1.OwnerReference{}

			handler.Handle(pod, updatedPod)

			Expect(logger.Logs()).ToNot(BeEmpty())
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
			updatedPod.OwnerReferences = append(updatedPod.OwnerReferences, metav1.OwnerReference{
				Kind: "extraterrestrial",
				Name: "E.T.",
			})
			handler.Handle(pod, updatedPod)

			Expect(routeEmitter.EmitArgsForCall(0)).To(Equal(eiriniroute.Message{
				Routes: eiriniroute.Routes{
					RegisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       8080,
				TLSPort:    0,
			}))
		})

		It("should not send the route if there's no StatefulSet owner", func() {
			updatedPod.OwnerReferences = []metav1.OwnerReference{
				{
					Kind: "extraterrestrial",
					Name: "E.T.",
				},
			}
			handler.Handle(pod, updatedPod)

			Expect(routeEmitter.EmitCallCount()).To(Equal(0))

			Expect(logger.Logs()).ToNot(BeEmpty())
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

		BeforeEach(func() {
			deletionTimestamp = metav1.Time{
				Time: time.Now(),
			}
			updatedPod.DeletionTimestamp = &deletionTimestamp
		})

		It("sends unregister route messages for the pod", func() {
			handler.Handle(pod, updatedPod)

			Expect(routeEmitter.EmitCallCount()).To(Equal(2))
			emittedRoutes := []eiriniroute.Message{
				routeEmitter.EmitArgsForCall(0),
				routeEmitter.EmitArgsForCall(1),
			}

			Expect(emittedRoutes).To(ConsistOf(eiriniroute.Message{
				Routes: eiriniroute.Routes{
					UnregisteredRoutes: []string{"mr-bombastic.50.60.70.80.nip.io"},
				},

				Name:       "mr-stateful-0-guid",
				InstanceID: "mr-stateful-0",
				Address:    "10.20.30.40",
				Port:       6565,
				TLSPort:    0,
			},
				eiriniroute.Message{
					Routes: eiriniroute.Routes{
						UnregisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io"},
					},

					Name:       "mr-stateful-0-guid",
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       8080,
					TLSPort:    0,
				},
			))
		})

		It("should provide a helpful error message", func() {
			handler.Handle(pod, updatedPod)

			Expect(logger.Logs()).ToNot(BeEmpty())

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
			BeforeEach(func() {
				updatedPod.Status.Conditions[0].Status = corev1.ConditionFalse
			})

			It("sends unregister route message for the pod", func() {
				handler.Handle(pod, updatedPod)

				Expect(routeEmitter.EmitCallCount()).To(Equal(2))
				emittedRoutes := []eiriniroute.Message{
					routeEmitter.EmitArgsForCall(0),
					routeEmitter.EmitArgsForCall(1),
				}

				Expect(emittedRoutes).To(ConsistOf(eiriniroute.Message{
					Routes: eiriniroute.Routes{
						UnregisteredRoutes: []string{"mr-bombastic.50.60.70.80.nip.io"},
					},

					Name:       "mr-stateful-0-guid",
					InstanceID: "mr-stateful-0",
					Address:    "10.20.30.40",
					Port:       6565,
					TLSPort:    0,
				},
					eiriniroute.Message{
						Routes: eiriniroute.Routes{
							UnregisteredRoutes: []string{"mr-stateful.50.60.70.80.nip.io"},
						},

						Name:       "mr-stateful-0-guid",
						InstanceID: "mr-stateful-0",
						Address:    "10.20.30.40",
						Port:       8080,
						TLSPort:    0,
					},
				))
			})

			It("should provide a helpful error message", func() {
				handler.Handle(pod, updatedPod)

				Expect(logger.Logs()).ToNot(BeEmpty())

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
				pod.Status.Conditions[0].Type = corev1.PodScheduled
				updatedPod.Status.Conditions[0].Type = corev1.PodInitialized
				handler.Handle(pod, updatedPod)

				Expect(routeEmitter.EmitCallCount()).To(Equal(0))
			})
		})

		Context("and the old pod readiness status is false", func() {
			It("should not send routes for the pod", func() {
				pod.Status.Conditions[0].Type = corev1.PodScheduled
				updatedPod.Status.Conditions[0].Status = corev1.ConditionFalse
				handler.Handle(pod, updatedPod)

				Expect(routeEmitter.EmitCallCount()).To(Equal(0))
			})
		})
	})
})
