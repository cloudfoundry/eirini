package rootfspatcher

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	apicore "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate counterfeiter . PodLister
type PodLister interface {
	List(opts meta.ListOptions) (*apicore.PodList, error)
}

type PodWaiter struct {
	PodLister         PodLister
	Logger            lager.Logger
	Timeout           time.Duration
	PodLabelSelector  string
	ExpectedPodLabels map[string]string
}

func (p PodWaiter) Wait() error {
	ready := make(chan interface{}, 1)
	defer close(ready)

	stop := make(chan interface{}, 1)
	defer close(stop)

	go p.poll(ready, stop)

	t := time.NewTimer(p.Timeout)
	if p.Timeout < 0 {
		return errors.New("provided timeout is not valid")
	}
	select {
	case <-ready:
		stop <- nil
		return nil
	case <-t.C:
		stop <- nil
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
	pods, err := p.PodLister.List(meta.ListOptions{LabelSelector: p.PodLabelSelector})

	if err != nil {
		p.Logger.Error("failed to list pods", err)
		return false
	}

	for _, pod := range pods.Items {
		if !p.expectedPodLabelsSet(pod) || !isReady(pod) {
			return false
		}
	}
	return true
}
func (p PodWaiter) expectedPodLabelsSet(pod apicore.Pod) bool {
	for key, expectedValue := range p.ExpectedPodLabels {
		actualValue, ok := pod.Labels[key]
		if !ok || expectedValue != actualValue {
			return false
		}
	}

	return true
}

func isReady(pod apicore.Pod) bool {
	return pod.Status.ContainerStatuses[0].Ready
}
