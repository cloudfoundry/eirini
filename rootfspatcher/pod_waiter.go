package rootfspatcher

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodWaiter struct {
	Timeout       time.Duration
	Client        clientcorev1.PodInterface
	RootfsVersion string
}

func (p PodWaiter) Wait() error {
	ready := make(chan interface{}, 1)
	defer close(ready)

	stop := make(chan interface{}, 1)
	defer close(stop)

	go p.poll(ready, stop)

	select {
	case <-ready:
		stop <- nil
		return nil
	case <-time.After(p.Timeout):
		return fmt.Errorf("timed out after %s", p.Timeout.String())
	}
}

func (p PodWaiter) poll(ready chan<- interface{}, stop <-chan interface{}) {
	for {
		select {
		case <-stop:
			return
		default:
			if p.podsUpdated() {
				ready <- nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (p PodWaiter) podsUpdated() bool {
	pods, _ := p.Client.List(metav1.ListOptions{})
	for _, pod := range pods.Items {
		if pod.Labels[RootfsVersionLabel] != p.RootfsVersion || !isRunningAndReady(pod) {
			return false
		}
	}
	return true
}

func isRunningAndReady(pod corev1.Pod) bool {
	return pod.Status.ContainerStatuses[0].Ready &&
		pod.Status.ContainerStatuses[0].State.Running != nil
}
