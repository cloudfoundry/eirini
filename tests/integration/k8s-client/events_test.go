package integration_test

import (
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
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
