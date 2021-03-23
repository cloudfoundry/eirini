package integration_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Secrets", func() {
	var secretClient *client.Secret

	BeforeEach(func() {
		secretClient = client.NewSecret(fixture.Clientset)
	})

	Describe("Get", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = tests.GenerateGUID()

			createSecret(fixture.Namespace, "foo", map[string]string{
				stset.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createSecret(extraNs, "foo", nil)
		})

		It("retrieves a Secret by namespace and name", func() {
			secret, err := secretClient.Get(ctx, fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(secret.Name).To(Equal("foo"))
			Expect(secret.Labels[stset.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("Create", func() {
		It("creates the secret in the namespace", func() {
			_, createErr := secretClient.Create(ctx, fixture.Namespace, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "very-secret",
				},
			})
			Expect(createErr).NotTo(HaveOccurred())

			secrets := listSecrets(fixture.Namespace)
			Expect(secretNames(secrets)).To(ContainElement("very-secret"))
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			createSecret(fixture.Namespace, "top-secret", map[string]string{"worst-year-ever": "2016"})
		})

		It("updates the existing secret", func() {
			_, err := secretClient.Update(ctx, fixture.Namespace, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "top-secret",
					Labels: map[string]string{
						"worst-year-ever": "2020",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			secret, err := getSecret(fixture.Namespace, "top-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Labels).To(HaveKeyWithValue("worst-year-ever", "2020"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createSecret(fixture.Namespace, "open-secret", nil)
		})

		It("deletes a Secret", func() {
			Eventually(func() []string {
				return secretNames(listSecrets(fixture.Namespace))
			}).Should(ContainElement("open-secret"))

			err := secretClient.Delete(ctx, fixture.Namespace, "open-secret")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string {
				return secretNames(listSecrets(fixture.Namespace))
			}).ShouldNot(ContainElement("open-secret"))
		})
	})
})

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

func podNames(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}

	return names
}

func statefulSetNames(statefulSets []appsv1.StatefulSet) []string {
	names := make([]string, 0, len(statefulSets))
	for _, statefulSet := range statefulSets {
		names = append(names, statefulSet.Name)
	}

	return names
}

func jobNames(jobs []batchv1.Job) []string {
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		names = append(names, job.Name)
	}

	return names
}

func jobGUIDs(allJobs []batchv1.Job) []string {
	guids := make([]string, 0, len(allJobs))
	for _, job := range allJobs {
		guids = append(guids, job.Labels[jobs.LabelGUID])
	}

	return guids
}

func secretNames(secrets []corev1.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, s := range secrets {
		names = append(names, s.Name)
	}

	return names
}

func eventNames(events []corev1.Event) []string {
	names := make([]string, 0, len(events))
	for _, e := range events {
		names = append(names, e.Name)
	}

	return names
}

func getJob(taskGUID string) (*batchv1.Job, error) {
	jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", jobs.LabelGUID, taskGUID),
	})
	if err != nil {
		return nil, err
	}

	if len(jobs.Items) != 1 {
		return nil, fmt.Errorf("expected 1 job with guid %s, got %d", taskGUID, len(jobs.Items))
	}

	return &jobs.Items[0], nil
}
