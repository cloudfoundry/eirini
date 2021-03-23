package event_test

import (
	"context"
	"errors"
	"strconv"
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/eirini/k8s/informers/event/eventfakes"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Event", func() {
	var (
		logger           *lagertest.TestLogger
		eventGenerator   *eventfakes.FakeCrashEventGenerator
		crashEmitter     *eventfakes.FakeCrashEmitter
		crashReconciler  *event.CrashReconciler
		controllerClient *reconcilerfakes.FakeClient
		pod              *corev1.Pod
		getPodError      error
		crashEvent       events.CrashEvent
		result           reconcile.Result
		err              error
	)

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Annotations: map[string]string{
					stset.AnnotationLastReportedAppCrash: "42",
				},
			},
		}
		getPodError = nil

		crashEvent = events.CrashEvent{
			ProcessGUID: "blahblah",
			AppCrashedRequest: cc_messages.AppCrashedRequest{
				CrashTimestamp: time.Now().Unix(),
			},
		}

		controllerClient = new(reconcilerfakes.FakeClient)

		logger = lagertest.NewTestLogger("crash-event-logger-test")

		eventGenerator = new(eventfakes.FakeCrashEventGenerator)
		eventGenerator.GenerateReturns(crashEvent, true)

		crashEmitter = new(eventfakes.FakeCrashEmitter)

		crashReconciler = event.NewCrashReconciler(
			logger,
			controllerClient,
			eventGenerator,
			crashEmitter,
		)
	})

	JustBeforeEach(func() {
		controllerClient.GetStub = func(c context.Context, nn types.NamespacedName, o client.Object) error {
			if getPodError != nil {
				return getPodError
			}

			p, ok := o.(*corev1.Pod)
			Expect(ok).To(BeTrue())
			p.Name = pod.Name
			p.Namespace = pod.Namespace
			p.Annotations = pod.Annotations

			return nil
		}

		result, err = crashReconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
		})
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).To(BeZero())
	})

	It("fetches the Pod", func() {
		Expect(controllerClient.GetCallCount()).To(Equal(1))

		_, namespacedName, _ := controllerClient.GetArgsForCall(0)
		Expect(namespacedName.Name).To(Equal(pod.Name))
		Expect(namespacedName.Namespace).To(Equal(pod.Namespace))
	})

	It("sends the correct args to the event generator", func() {
		Expect(eventGenerator.GenerateCallCount()).To(Equal(1))

		_, inputPod, inputLogger := eventGenerator.GenerateArgsForCall(0)
		Expect(inputPod).To(Equal(pod))
		Expect(inputLogger).To(Equal(logger))
	})

	It("sends a crash event", func() {
		Eventually(crashEmitter.EmitCallCount).Should(Equal(1))

		actualevent := crashEmitter.EmitArgsForCall(0)
		Expect(actualevent.ProcessGUID).To(Equal("blahblah"))
	})

	It("sets the last reported crash timestamp on pod annotations", func() {
		Expect(controllerClient.PatchCallCount()).To(Equal(1))
		_, p, patch, _ := controllerClient.PatchArgsForCall(0)

		pd, ok := p.(*corev1.Pod)
		Expect(ok).To(BeTrue(), "didn't pass a *Pod to patch")

		Expect(pd.Name).To(Equal(pod.Name))
		Expect(pd.Namespace).To(Equal(pod.Namespace))

		var patchBytes []byte
		patchBytes, err = patch.Data(p)
		Expect(err).NotTo(HaveOccurred())
		timestamp := strconv.FormatInt(crashEvent.CrashTimestamp, 10)
		Expect(string(patchBytes)).To(SatisfyAll(
			ContainSubstring(stset.AnnotationLastReportedAppCrash),
			ContainSubstring(timestamp),
		))
	})

	When("the app does not have to be reported", func() {
		BeforeEach(func() {
			eventGenerator.GenerateReturns(crashEvent, false)
		})

		It("does NOT send a crash event", func() {
			Expect(crashEmitter.EmitCallCount()).To(Equal(0))
		})
	})

	When("the app crash has not previously crashed", func() {
		BeforeEach(func() {
			pod.Annotations = nil
			eventGenerator.GenerateReturns(crashEvent, true)
		})

		It("sends a crash event", func() {
			Expect(crashEmitter.EmitCallCount()).To(Equal(1))
		})
	})

	When("the app crash has already been reported", func() {
		BeforeEach(func() {
			pod.Annotations[stset.AnnotationLastReportedAppCrash] = "123456"
			crashEvent.CrashTimestamp = 123456
			eventGenerator.GenerateReturns(crashEvent, true)
		})

		It("does NOT send a crash event", func() {
			Expect(crashEmitter.EmitCallCount()).To(Equal(0))
		})
	})

	When("the Pod doesn't exist", func() {
		BeforeEach(func() {
			getPodError = apierrors.NewNotFound(schema.GroupResource{}, "")
		})

		It("does not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
		})

		It("does not emit an event", func() {
			Expect(crashEmitter.EmitCallCount()).To(BeZero())
		})
	})

	When("getting the Pod fails", func() {
		BeforeEach(func() {
			getPodError = errors.New("get-pod-error")
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("get-pod-error")))
		})

		It("does not emit an event", func() {
			Expect(crashEmitter.EmitCallCount()).To(BeZero())
		})
	})

	When("emitting the event fails", func() {
		BeforeEach(func() {
			crashEmitter.EmitReturns(errors.New("emit-error"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("emit-error")))
		})
	})
})
