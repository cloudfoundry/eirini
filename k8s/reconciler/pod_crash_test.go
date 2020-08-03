package reconciler_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
		controllerClient.GetStub = func(c context.Context, nn types.NamespacedName, o runtime.Object) error {
			pod := o.(*corev1.Pod)
			pod.Namespace = "some-ns"
			pod.Name = "app-instance"
			pod.OwnerReferences = podOwners

			return podGetError
		}
		_, resultErr = podCrashReconciler.Reconcile(reconcile.Request{
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
		pod, _ := crashEventGenerator.GenerateArgsForCall(0)
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
		var (
			timestamp time.Time
		)

		BeforeEach(func() {
			timestamp = time.Unix(time.Now().Unix(), 0)
			crashEventGenerator.GenerateReturns(events.CrashEvent{
				ProcessGUID: "process-guid",
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Instance:        "instance-name",
					Index:           3,
					CellID:          "cell-id",
					Reason:          "it crashed",
					ExitStatus:      1,
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
			_, event, _ := eventsClient.CreateArgsForCall(0)
			Expect(event.Namespace).To(Equal("some-ns"))
			Expect(event.GenerateName).To(Equal("instance-name"))
			Expect(event.Labels).To(HaveKeyWithValue("cloudfoundry.org/instance_index", "3"))
			Expect(event.Annotations).To(HaveKeyWithValue("cloudfoundry.org/process_guid", "process-guid"))

			Expect(event.LastTimestamp).To(Equal(metav1.NewTime(timestamp)))
			Expect(event.FirstTimestamp).To(Equal(metav1.NewTime(timestamp)))
			Expect(event.EventTime).To(Equal(metav1.NewMicroTime(timestamp)))
			Expect(event.InvolvedObject).To(Equal(corev1.ObjectReference{
				Kind:            "LRP",
				Namespace:       "some-ns",
				Name:            "parent-lrp",
				UID:             "asdf",
				APIVersion:      "eirini/v1",
				ResourceVersion: "",
				FieldPath:       "spec.containers{opi}",
			}))
			Expect(event.Reason).To(Equal("it crashed"))
			Expect(event.Message).To(Equal("exit code: 1, message: oops"))
			Expect(event.Count).To(Equal(int32(1)))
			Expect(event.Source).To(Equal(corev1.EventSource{Component: "eirini-controller"}))
			Expect(event.Type).To(Equal("Warning"))
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
