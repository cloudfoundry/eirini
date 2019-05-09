package rootfspatcher_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	. "code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("PodWaiter", func() {
	var (
		client    *fake.Clientset
		pod       corev1.Pod
		logger    *lagertest.TestLogger
		namespace string
		waiter    PodWaiter
	)
	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		namespace = "test-ns"
		pod = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-pod",
				Labels: map[string]string{RootfsVersionLabel: "version1"},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{
					Ready: false,
				}},
			},
		}
		_, err := client.CoreV1().Pods(namespace).Create(&pod)
		Expect(err).ToNot(HaveOccurred())
		logger = lagertest.NewTestLogger("test")
		waiter = PodWaiter{
			Client:        client.CoreV1().Pods(namespace),
			Logger:        logger,
			RootfsVersion: "version2",
			Timeout:       1 * time.Second,
		}
	})

	It("should finish when all pods get the new version and are ready", func() {
		channel := make(chan error, 1)
		defer close(channel)

		go func(ch chan<- error) {
			err := waiter.Wait()
			ch <- err
		}(channel)

		updatedPod := pod.DeepCopy()
		updatedPod.Labels[RootfsVersionLabel] = "version2"
		updatedPod.Status.ContainerStatuses[0].Ready = true
		updatedPod.Status.ContainerStatuses[0].State.Running = &corev1.ContainerStateRunning{}
		_, err := client.CoreV1().Pods(namespace).Update(updatedPod)
		Expect(err).ToNot(HaveOccurred())

		Eventually(channel).Should(Receive(nil))
	})

	It("should timeout if a pod doesn't become Ready", func() {
		channel := make(chan error, 1)
		defer close(channel)

		go func(ch chan<- error) {
			err := waiter.Wait()
			ch <- err
		}(channel)

		updatedPod := pod.DeepCopy()
		updatedPod.Labels[RootfsVersionLabel] = "version2"
		updatedPod.Status.ContainerStatuses[0].State.Running = &corev1.ContainerStateRunning{}
		_, err := client.CoreV1().Pods(namespace).Update(updatedPod)
		Expect(err).ToNot(HaveOccurred())

		Eventually(channel, "2s").Should(Receive(MatchError("timed out after 1s")))
	})

	It("should timeout if a pod doesn't become Running", func() {
		channel := make(chan error, 1)
		defer close(channel)

		go func(ch chan<- error) {
			err := waiter.Wait()
			ch <- err
		}(channel)

		updatedPod := pod.DeepCopy()
		updatedPod.Labels[RootfsVersionLabel] = "version2"
		updatedPod.Status.ContainerStatuses[0].Ready = true
		_, err := client.CoreV1().Pods(namespace).Update(updatedPod)
		Expect(err).ToNot(HaveOccurred())

		Eventually(channel, "2s").Should(Receive(MatchError("timed out after 1s")))
	})

	It("should timeout if all pods fail to get the new version and are ready within specified time", func() {
		channel := make(chan error, 1)
		defer close(channel)

		go func(ch chan<- error) {
			err := waiter.Wait()
			ch <- err
		}(channel)

		Eventually(channel, "2s").Should(Receive(MatchError("timed out after 1s")))
	})

	It("should not timeout if timeout duration is -1", func() {
		channel := make(chan error, 1)
		defer close(channel)

		go func(ch chan<- error) {
			waiter.Timeout = -1
			err := waiter.Wait()
			fmt.Fprintf(GinkgoWriter, "waiter response: %#v", err)
			ch <- err
		}(channel)

		Consistently(channel, "2s").ShouldNot(Receive())

		updatedPod := pod.DeepCopy()
		updatedPod.Labels[RootfsVersionLabel] = "version2"
		updatedPod.Status.ContainerStatuses[0].Ready = true
		updatedPod.Status.ContainerStatuses[0].State.Running = &corev1.ContainerStateRunning{}
		_, err := client.CoreV1().Pods(namespace).Update(updatedPod)
		Expect(err).ToNot(HaveOccurred())

		Eventually(channel, "2s").Should(Receive(nil))
	})

	It("should log if listing pods fails", func() {
		client.PrependReactor("list", "pods", func(action testing.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("made up failure")
		})

		err := waiter.Wait()
		Expect(err).To(HaveOccurred())
		Expect(logger.LogMessages()).To(ContainElement("test.failed to list pods"))
		Expect(logger.Logs()[0].Data["error"]).To(Equal("made up failure"))
	})
})
