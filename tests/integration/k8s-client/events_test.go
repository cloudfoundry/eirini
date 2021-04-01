package integration_test

import (
	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Events", func() {
	var eventClient *client.Event

	BeforeEach(func() {
		eventClient = client.NewEvent(fixture.Clientset)
	})

	Describe("GetByPod", func() {
		var pod corev1.Pod

		BeforeEach(func() {
			pod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "the-pod",
					Namespace: fixture.Namespace,
					UID:       types.UID(tests.GenerateGUID()),
				},
			}

			createEvent(fixture.Namespace, "the-event", corev1.ObjectReference{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       pod.UID,
			})

			createEvent(fixture.Namespace, "another-event", corev1.ObjectReference{
				Name:      "another-pod",
				Namespace: fixture.Namespace,
				UID:       types.UID(tests.GenerateGUID()),
			})
		})

		It("lists the events beloging to a pod", func() {
			Eventually(func() []string {
				events, err := eventClient.GetByPod(ctx, pod)
				Expect(err).NotTo(HaveOccurred())

				return eventNames(events)
			}).Should(ConsistOf("the-event"))
		})
	})

	Describe("GetByInstanceAndReason", func() {
		var ownerRef corev1.ObjectReference

		BeforeEach(func() {
			ownerRef = corev1.ObjectReference{
				Name:      "the-owner",
				Namespace: fixture.Namespace,
				UID:       types.UID(tests.GenerateGUID()),
				Kind:      "the-kind",
			}

			createCrashEvent(fixture.Namespace, "the-crash-event", ownerRef, events.CrashEvent{
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Index:  42,
					Reason: "the-reason",
				},
			})

			createCrashEvent(fixture.Namespace, "another-crash-event", ownerRef, events.CrashEvent{
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Index:  43,
					Reason: "the-reason",
				},
			})

			createCrashEvent(fixture.Namespace, "yet-another-crash-event", ownerRef, events.CrashEvent{
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Index:  42,
					Reason: "another-reason",
				},
			})
		})

		It("lists the events beloging to an LRP instance with a specific reason", func() {
			Eventually(func() string {
				event, err := eventClient.GetByInstanceAndReason(ctx, fixture.Namespace, metav1.OwnerReference{
					Name: ownerRef.Name,
					UID:  ownerRef.UID,
					Kind: ownerRef.Kind,
				}, 42, "the-reason")
				Expect(err).NotTo(HaveOccurred())

				return event.Name
			}).Should(Equal("the-crash-event"))
		})
	})

	Describe("Create", func() {
		It("creates the secret in the namespace", func() {
			_, createErr := eventClient.Create(ctx, fixture.Namespace, &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a-party",
					Namespace: fixture.Namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Namespace: fixture.Namespace,
				},
			})
			Expect(createErr).NotTo(HaveOccurred())

			events := listEvents(fixture.Namespace)

			Expect(eventNames(events)).To(ContainElement("a-party"))
		})
	})

	Describe("Update", func() {
		var event *corev1.Event

		BeforeEach(func() {
			event = createEvent(fixture.Namespace, "whatever", corev1.ObjectReference{Namespace: fixture.Namespace})
		})

		It("updates the existing secret", func() {
			event.Reason = "the reason"
			_, err := eventClient.Update(ctx, fixture.Namespace, event)
			Expect(err).NotTo(HaveOccurred())

			event, err := getEvent(fixture.Namespace, "whatever")

			Expect(err).NotTo(HaveOccurred())
			Expect(event.Reason).To(Equal("the reason"))
		})
	})
})

func eventNames(events []corev1.Event) []string {
	names := make([]string, 0, len(events))
	for _, e := range events {
		names = append(names, e.Name)
	}

	return names
}
