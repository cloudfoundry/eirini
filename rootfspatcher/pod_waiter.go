package rootfspatcher

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
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

	t := time.NewTimer(p.Timeout)
	if p.Timeout < 0 {
		t.Stop()
	}
	select {
	case <-ready:
		stop <- nil
		return nil
	case <-t.C:
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
		if pod.Labels[RootfsVersionLabel] != p.RootfsVersion || !isRunning(pod) {
			return false
		}
	}
	return true
}

func isRunning(pod corev1.Pod) bool {
	state := utils.GetPodState(pod)
	return state == opi.RunningState
}
