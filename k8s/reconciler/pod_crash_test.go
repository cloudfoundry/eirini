package reconciler_test

import (
	"context"
	"strconv"
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("K8s/Reconciler/AppCrash", func() {
	var (
		podCrashReconciler  *reconciler.PodCrash
		controllerClient    *reconcilerfakes.FakeClient
		crashEventGenerator *reconcilerfakes.FakeCrashEventGenerator
		eventsClient        *reconcilerfakes.FakeEventsClient
		statefulSetGetter   *reconcilerfakes.FakeStatefulSetGetter
		resultErr           error
		podOwners           []metav1.OwnerReference
		podGetError         error
		podAnnotations      map[string]string
	)

	BeforeEach(func() {
		controllerClient = new(reconcilerfakes.FakeClient)
		crashEventGenerator = new(reconcilerfakes.FakeCrashEventGenerator)
		eventsClient = new(reconcilerfakes.FakeEventsClient)
		eventsClient.CreateReturns(&corev1.Event{}, nil)
		statefulSetGetter = new(reconcilerfakes.FakeStatefulSetGetter)
		podCrashReconciler = reconciler.NewPodCrash(
			lagertest.NewTestLogger("pod-crash-test"),
			controllerClient,
			crashEventGenerator,
			eventsClient,
			statefulSetGetter,
		)
		podAnnotations = nil

		statefulSet := appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "eirini/v1",
						Kind:       "LRP",
						Name:       "parent-lrp",
						UID:        "asdf",
					},
				},
			},
		}
		statefulSetGetter.GetReturns(&statefulSet, nil)
		eventsClient.GetByInstanceAndReasonReturns(nil, nil)

		podGetError = nil
		t := true
		podOwners = []metav1.OwnerReference{
			{
				Name:       "stateful-set-controller",
				UID:        "sdfp",
				Controller: &t,
			},
			{
				APIVersion: "v1",
				Kind:       "StatefulSet",
				Name:       "parent-statefulset",
				UID:        "sdfp",
			},
		}
	})

	JustBeforeEach(func() {
		controllerClient.GetStub = func(c context.Context, nn types.NamespacedName, o client.Object) error {
			pod, ok := o.(*corev1.Pod)
			Expect(ok).To(BeTrue())
			pod.Namespace = "some-ns"
			pod.Name = "app-instance"
			pod.OwnerReferences = podOwners
			pod.Annotations = podAnnotations

			return podGetError
		}
		_, resultErr = podCrashReconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "some-ns",
				Name:      "app-instance",
			},
		})
	})

	It("sends the correct pod info to the crash event generator", func() {
		Expect(resultErr).NotTo(HaveOccurred())
		Expect(controllerClient.GetCallCount()).To(Equal(1))
		_, nsName, _ := controllerClient.GetArgsForCall(0)
		Expect(nsName).To(Equal(types.NamespacedName{Namespace: "some-ns", Name: "app-instance"}))

		Expect(crashEventGenerator.GenerateCallCount()).To(Equal(1))
		_, pod, _ := crashEventGenerator.GenerateArgsForCall(0)
		Expect(pod.Namespace).To(Equal("some-ns"))
		Expect(pod.Name).To(Equal("app-instance"))
	})

	When("no crash is generated", func() {
		BeforeEach(func() {
			crashEventGenerator.GenerateReturns(events.CrashEvent{}, false)
		})

		It("does not create a k8s event", func() {
			Expect(eventsClient.CreateCallCount()).To(Equal(0))
		})
	})

	When("a crash has occurred", func() {
		var timestamp time.Time

		BeforeEach(func() {
			timestamp = time.Unix(time.Now().Unix(), 0)
			crashEventGenerator.GenerateReturns(events.CrashEvent{
				ProcessGUID: "process-guid",
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Instance:        "instance-name",
					Index:           3,
					Reason:          "Error",
					ExitStatus:      6,
					ExitDescription: "oops",
					CrashTimestamp:  timestamp.Unix(),
				},
			}, true)
		})

		It("looks up the LRP via the stateful set", func() {
			Expect(statefulSetGetter.GetCallCount()).To(Equal(1))
		})

		It("creates a k8s event", func() {
			Expect(eventsClient.CreateCallCount()).To(Equal(1))
			_, namespace, event := eventsClient.CreateArgsForCall(0)
			Expect(namespace).To(Equal("some-ns"))
			Expect(event.GenerateName).To(Equal("instance-name-"))
			Expect(event.Labels).To(HaveKeyWithValue("cloudfoundry.org/instance_index", "3"))
			Expect(event.Annotations).To(HaveKeyWithValue("cloudfoundry.org/process_guid", "process-guid"))

			Expect(event.LastTimestamp).To(Equal(metav1.NewTime(timestamp)))
			Expect(event.FirstTimestamp).To(Equal(metav1.NewTime(timestamp)))
			Expect(event.EventTime.Time).To(SatisfyAll(
				BeTemporally(">", timestamp),
				BeTemporally("<", time.Now()),
			))
			Expect(event.InvolvedObject).To(Equal(corev1.ObjectReference{
				Kind:            "LRP",
				Namespace:       "some-ns",
				Name:            "parent-lrp",
				UID:             "asdf",
				APIVersion:      "eirini/v1",
				ResourceVersion: "",
				FieldPath:       "spec.containers{opi}",
			}))
			Expect(event.Reason).To(Equal("Container: Error"))
			Expect(event.Message).To(Equal("Container terminated with exit code: 6"))
			Expect(event.Count).To(Equal(int32(1)))
			Expect(event.Source).To(Equal(corev1.EventSource{Component: "eirini-controller"}))
			Expect(event.Type).To(Equal("Warning"))
		})

		It("records the crash timestamp as an annotation on the pod", func() {
			Expect(controllerClient.PatchCallCount()).To(Equal(1))

			_, p, patch, _ := controllerClient.PatchArgsForCall(0)

			pd, ok := p.(*corev1.Pod)
			Expect(ok).To(BeTrue(), "didn't pass a *Pod to patch")

			Expect(pd.Name).To(Equal("app-instance"))
			Expect(pd.Namespace).To(Equal("some-ns"))

			patchBytes, err := patch.Data(p)
			Expect(err).NotTo(HaveOccurred())
			patchTimestamp := strconv.FormatInt(timestamp.Unix(), 10)
			Expect(string(patchBytes)).To(SatisfyAll(
				ContainSubstring(stset.AnnotationLastReportedLRPCrash),
				ContainSubstring(patchTimestamp),
			))
		})

		When("the app crash has already been reported", func() {
			BeforeEach(func() {
				podAnnotations = map[string]string{
					stset.AnnotationLastReportedLRPCrash: strconv.FormatInt(timestamp.Unix(), 10),
				}
			})

			It("does not requeue", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})

			It("does not create an event", func() {
				Expect(eventsClient.CreateCallCount()).To(Equal(0))
			})
		})

		When("the crashed pod does not have a StatefulSet owner", func() {
			BeforeEach(func() {
				podOwners = podOwners[:len(podOwners)-1]
			})

			It("does not requeue", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})

			It("does not create an event", func() {
				Expect(statefulSetGetter.GetCallCount()).To(Equal(0))
				Expect(eventsClient.CreateCallCount()).To(Equal(0))
			})
		})

		When("the associated stateful set doesn't have a LRP owner", func() {
			BeforeEach(func() {
				statefulSetGetter.GetReturns(&appsv1.StatefulSet{}, nil)
			})

			It("does not requeue", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})

			It("does not create an event", func() {
				Expect(eventsClient.CreateCallCount()).To(Equal(0))
			})
		})

		When("the statefulset getter fails to get", func() {
			BeforeEach(func() {
				statefulSetGetter.GetReturns(&appsv1.StatefulSet{}, errors.New("boom"))
			})

			It("requeues the request", func() {
				Expect(resultErr).To(HaveOccurred())
			})

			It("does not create an event", func() {
				Expect(eventsClient.CreateCallCount()).To(Equal(0))
			})
		})

		When("creating the event errors", func() {
			BeforeEach(func() {
				eventsClient.CreateReturns(nil, errors.New("boom"))
			})

			It("requeues the request", func() {
				Expect(resultErr).To(MatchError(ContainSubstring("failed to create event")))
			})
		})
	})

	When("a crash has occurred multiple times", func() {
		var (
			timestampFirst  time.Time
			timestampSecond time.Time
			eventTime       metav1.MicroTime
		)

		BeforeEach(func() {
			timestampFirst = time.Unix(time.Now().Unix(), 0)
			timestampSecond = timestampFirst.Add(10 * time.Second)

			crashEventGenerator.GenerateReturns(events.CrashEvent{
				ProcessGUID: "process-guid",
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Instance:        "instance-name",
					Index:           3,
					Reason:          "Error",
					ExitStatus:      6,
					ExitDescription: "oops",
					CrashTimestamp:  timestampFirst.Unix(),
				},
			}, true)

			crashEventGenerator.GenerateReturnsOnCall(1, events.CrashEvent{
				ProcessGUID: "process-guid",
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Instance:        "instance-name",
					Index:           3,
					Reason:          "Error",
					ExitStatus:      6,
					ExitDescription: "oops",
					CrashTimestamp:  timestampSecond.Unix(),
				},
			}, true)

			eventTime = metav1.MicroTime{Time: timestampFirst.Add(time.Second)}
			eventsClient.GetByInstanceAndReasonReturnsOnCall(1, &corev1.Event{
				ObjectMeta:     metav1.ObjectMeta{Name: "instance-name", Namespace: "some-ns"},
				Count:          1,
				Reason:         "Container: Error",
				Message:        "Container terminated with exit code: 6",
				FirstTimestamp: metav1.Time{Time: timestampFirst},
				LastTimestamp:  metav1.Time{Time: timestampFirst},
				EventTime:      eventTime,
			}, nil)
		})

		It("updates the existing event", func() {
			_, resultErr = podCrashReconciler.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "some-ns",
					Name:      "app-instance",
				},
			})

			Expect(eventsClient.GetByInstanceAndReasonCallCount()).To(Equal(2))
			_, namespace, ownerRef, instanceIndex, reason := eventsClient.GetByInstanceAndReasonArgsForCall(0)
			Expect(namespace).To(Equal("some-ns"))
			Expect(ownerRef.Kind).To(Equal("LRP"))
			Expect(ownerRef.Name).To(Equal("parent-lrp"))
			Expect(instanceIndex).To(Equal(3))
			Expect(reason).To(Equal("Container: Error"))

			Expect(eventsClient.UpdateCallCount()).To(Equal(1))
			_, ns, event := eventsClient.UpdateArgsForCall(0)

			Expect(ns).To(Equal("some-ns"))
			Expect(event.Reason).To(Equal("Container: Error"))
			Expect(event.Message).To(Equal("Container terminated with exit code: 6"))
			Expect(event.Count).To(BeNumerically("==", 2))
			Expect(event.FirstTimestamp).To(Equal(metav1.NewTime(timestampFirst)))
			Expect(event.LastTimestamp).To(Equal(metav1.NewTime(timestampSecond)))
			Expect(event.EventTime).To(Equal(eventTime))
		})

		When("listing events errors", func() {
			BeforeEach(func() {
				eventsClient.GetByInstanceAndReasonReturns(nil, errors.New("oof"))
			})

			It("does not create an event", func() {
				Expect(eventsClient.CreateCallCount()).To(Equal(0))
			})

			It("requeues the request", func() {
				Expect(resultErr).To(HaveOccurred())
			})
		})

		When("updating the event errors", func() {
			BeforeEach(func() {
				eventsClient.GetByInstanceAndReasonReturns(&corev1.Event{}, nil)
				eventsClient.UpdateReturns(nil, errors.New("oof"))
			})

			It("requeues the request", func() {
				Expect(resultErr).To(HaveOccurred())
			})
		})
	})

	When("getting the pod errors", func() {
		BeforeEach(func() {
			podGetError = errors.New("boom")
		})

		It("does not create an event", func() {
			Expect(statefulSetGetter.GetCallCount()).To(Equal(0))
			Expect(eventsClient.CreateCallCount()).To(Equal(0))
		})

		It("requeues the request", func() {
			Expect(resultErr).To(HaveOccurred())
		})

		When("it returns a not found error", func() {
			BeforeEach(func() {
				podGetError = apierrors.NewNotFound(schema.GroupResource{}, "my-pod")
			})

			It("does not create an event", func() {
				Expect(statefulSetGetter.GetCallCount()).To(Equal(0))
				Expect(eventsClient.CreateCallCount()).To(Equal(0))
			})

			It("does not requeue the request", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})
		})
	})
})
