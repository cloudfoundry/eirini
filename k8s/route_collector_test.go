package k8s_test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RouteCollector", func() {
	var (
		pods               []corev1.Pod
		statefulsets       []appsv1.StatefulSet
		podsGetter         *k8sfakes.FakePodsGetter
		statefulSetGetter  *k8sfakes.FakeStatefulSetGetter
		statefulset1       appsv1.StatefulSet
		statefulset2       appsv1.StatefulSet
		pod11              corev1.Pod
		pod21              corev1.Pod
		pod22              corev1.Pod
		routeMessages      []route.Message
		collector          RouteCollector
		logger             *lagertest.TestLogger
		err                error
		getStatefulSetsErr error
		getPodsErr         error
	)

	createPod := func(name string, ssName string, ip string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					stset.LabelGUID: fmt.Sprintf("%s-guid", name),
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       ssName,
				}},
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
	createStatefulSet := func(name string, routes string) appsv1.StatefulSet {
		return appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					stset.AnnotationRegisteredRoutes: routes,
				},
			},
		}
	}

	BeforeEach(func() {
		routes1, marshalErr := json.Marshal([]cf.Route{{Hostname: "foo.example.com", Port: 80}})
		Expect(marshalErr).ToNot(HaveOccurred())
		routes2, marshalErr := json.Marshal([]cf.Route{{Hostname: "bar.example.com", Port: 9000}})
		Expect(marshalErr).ToNot(HaveOccurred())

		statefulset1 = createStatefulSet("ss-1", string(routes1))
		statefulset2 = createStatefulSet("ss-2", string(routes2))
		pod11 = createPod("pod-11", "ss-1", "10.0.0.1")
		pod21 = createPod("pod-21", "ss-2", "10.0.0.2")
		pod22 = createPod("pod-22", "ss-2", "10.0.0.3")
		pods = []corev1.Pod{}
		statefulsets = []appsv1.StatefulSet{}
		getPodsErr = nil
		getStatefulSetsErr = nil

		podsGetter = new(k8sfakes.FakePodsGetter)
		statefulSetGetter = new(k8sfakes.FakeStatefulSetGetter)
		logger = lagertest.NewTestLogger("collector-test")
		collector = NewRouteCollector(podsGetter, statefulSetGetter, logger)
	})

	JustBeforeEach(func() {
		podsGetter.GetAllReturns(pods, getPodsErr)
		statefulSetGetter.GetBySourceTypeReturns(statefulsets, getStatefulSetsErr)
		routeMessages, err = collector.Collect(ctx)
	})

	It("should not return anything if there are no pods or statefulsets", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(routeMessages).To(BeEmpty())
	})

	Context("when there are pods and statefulsets", func() {
		BeforeEach(func() {
			pods = []corev1.Pod{pod11, pod21, pod22}
			statefulsets = []appsv1.StatefulSet{statefulset1, statefulset2}
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return routes to be registered", func() {
			Expect(routeMessages).To(ConsistOf([]route.Message{
				{
					InstanceID: "pod-11",
					Name:       "pod-11-guid",
					Address:    "10.0.0.1",
					Port:       80,
					TLSPort:    0,
					Routes: route.Routes{
						RegisteredRoutes: []string{"foo.example.com"},
					},
				},
				{
					InstanceID: "pod-21",
					Name:       "pod-21-guid",
					Address:    "10.0.0.2",
					Port:       9000,
					TLSPort:    0,
					Routes: route.Routes{
						RegisteredRoutes: []string{"bar.example.com"},
					},
				},
				{
					InstanceID: "pod-22",
					Name:       "pod-22-guid",
					Address:    "10.0.0.3",
					Port:       9000,
					TLSPort:    0,
					Routes: route.Routes{
						RegisteredRoutes: []string{"bar.example.com"},
					},
				},
			}))
		})

		Context("and a pod has multiple routes", func() {
			BeforeEach(func() {
				routes, marshalErr := json.Marshal([]cf.Route{
					{Hostname: "foo.example.com", Port: 80},
					{Hostname: "bar.example.com", Port: 443},
				})
				Expect(marshalErr).ToNot(HaveOccurred())
				statefulsets[0].Annotations[stset.AnnotationRegisteredRoutes] = string(routes)
			})

			It("should return a route message for each registered route", func() {
				Expect(routeMessages).To(ContainElement(route.Message{
					InstanceID: "pod-11",
					Name:       "pod-11-guid",
					Address:    "10.0.0.1",
					Port:       80,
					TLSPort:    0,
					Routes: route.Routes{
						RegisteredRoutes: []string{"foo.example.com"},
					},
				}))
				Expect(routeMessages).To(ContainElement(route.Message{
					InstanceID: "pod-11",
					Name:       "pod-11-guid",
					Address:    "10.0.0.1",
					Port:       443,
					TLSPort:    0,
					Routes: route.Routes{
						RegisteredRoutes: []string{"bar.example.com"},
					},
				}))
			})
		})

		Context("and there are pods that are not ready", func() {
			BeforeEach(func() {
				pods[0].Status.Conditions[0].Status = corev1.ConditionFalse
			})

			It("should not return routes to be registered for pods which are not ready", func() {
				Expect(routeMessages).To(ConsistOf([]route.Message{
					{
						InstanceID: "pod-21",
						Name:       "pod-21-guid",
						Address:    "10.0.0.2",
						Port:       9000,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"bar.example.com"},
						},
					},
					{
						InstanceID: "pod-22",
						Name:       "pod-22-guid",
						Address:    "10.0.0.3",
						Port:       9000,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"bar.example.com"},
						},
					},
				}))
			})
		})

		Context("and there is a pod which has no condition statuses", func() {
			BeforeEach(func() {
				pods[0].Status.Conditions[0].Type = corev1.PodInitialized
			})

			It("should not register any routes for the pod", func() {
				Expect(routeMessages).To(ConsistOf([]route.Message{
					{
						InstanceID: "pod-21",
						Name:       "pod-21-guid",
						Address:    "10.0.0.2",
						Port:       9000,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"bar.example.com"},
						},
					},
					{
						InstanceID: "pod-22",
						Name:       "pod-22-guid",
						Address:    "10.0.0.3",
						Port:       9000,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"bar.example.com"},
						},
					},
				}))
			})
		})

		Context("and there is a pod without an owner", func() {
			BeforeEach(func() {
				pod11.OwnerReferences = nil
				pods = []corev1.Pod{pod11}
			})

			It("should not register any routes for the pod", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(routeMessages).To(BeEmpty())
			})
		})

		Context("and there is a pod owned by a nonexistent statefulset", func() {
			BeforeEach(func() {
				pod11.OwnerReferences[0].Name = "does-not-exist"
				pods = []corev1.Pod{pod11}
			})

			It("should not register any routes for the pod", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(routeMessages).To(BeEmpty())
			})

			It("should provide a helpful log message", func() {
				logs := logger.Logs()
				Expect(logs).To(HaveLen(1))
				log := logs[0]
				Expect(log.Message).To(Equal("collector-test.collect.failed-to-get-routes"))
				Expect(log.LogLevel).To(Equal(lager.DEBUG))
				Expect(log.Data).To(HaveKeyWithValue("error", "statefulset for pod pod-11 not found"))
			})
		})

		Context("and there is a pod not owned by a statefulset", func() {
			BeforeEach(func() {
				pod11.OwnerReferences[0].Kind = "Job"
				pods = []corev1.Pod{pod11}
			})

			It("should not register any routes for the pod", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(routeMessages).To(BeEmpty())
			})
		})

		Context("and there is a pod with multiple owners as long as one is a StatefulSet", func() {
			BeforeEach(func() {
				pod11.OwnerReferences[0].Kind = "NotStatefulSet"
				pod11.OwnerReferences[0].Name = "not-ss-1"
				pod11.OwnerReferences = append(pod11.OwnerReferences, metav1.OwnerReference{
					Kind: "StatefulSet",
					Name: statefulset1.Name,
				})
				pods = []corev1.Pod{pod11}
			})

			It("should return route", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(routeMessages).To(Equal([]route.Message{
					{
						InstanceID: "pod-11",
						Name:       "pod-11-guid",
						Address:    "10.0.0.1",
						Port:       80,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"foo.example.com"},
						},
					},
				}))
			})
		})

		Context("and a statefulset has no RegisteredRoutes", func() {
			BeforeEach(func() {
				statefulsets[1].Annotations = map[string]string{}
			})

			It("should not register routes for that statefulset", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(routeMessages).To(Equal([]route.Message{
					{
						InstanceID: "pod-11",
						Name:       "pod-11-guid",
						Address:    "10.0.0.1",
						Port:       80,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"foo.example.com"},
						},
					},
				}))
			})
		})

		Context("and a statefulset has invalid routes", func() {
			BeforeEach(func() {
				statefulsets[1].Annotations[stset.AnnotationRegisteredRoutes] = "{invalid json"
			})

			It("should not return routes for that statefulset", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(routeMessages).To(Equal([]route.Message{
					{
						InstanceID: "pod-11",
						Name:       "pod-11-guid",
						Address:    "10.0.0.1",
						Port:       80,
						TLSPort:    0,
						Routes: route.Routes{
							RegisteredRoutes: []string{"foo.example.com"},
						},
					},
				}))
			})
		})

		Context("when listing pods fails", func() {
			BeforeEach(func() {
				pods = nil
				getPodsErr = errors.New("boom")
			})

			It("should return error if listing pods fails", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to list pods: boom")))
			})
		})

		Context("when listing statefulsets fails", func() {
			BeforeEach(func() {
				statefulsets = nil
				getStatefulSetsErr = errors.New("boom")
			})

			It("should return error if listing statefulsets fails", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets: boom")))
			})
		})
	})
})
