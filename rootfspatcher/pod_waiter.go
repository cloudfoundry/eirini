package rootfspatcher

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodWaiter struct {
	Client        clientcorev1.PodInterface
	Logger        lager.Logger
	RootfsVersion string
	Timeout       time.Duration
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
	pods, err := p.Client.List(metav1.ListOptions{})

	if err != nil {
		p.Logger.Error("failed to list pods", err)
		return false
	}

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
