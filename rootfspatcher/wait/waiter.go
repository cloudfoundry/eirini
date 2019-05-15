package wait

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

type PodsRunning struct {
	PodLister PodLister
	Logger    lager.Logger
	Timeout   time.Duration
	Label     string
}

func (p PodsRunning) Wait() error {
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

func (p PodsRunning) poll(ready chan<- interface{}, stop <-chan interface{}) {
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

func (p PodsRunning) podsUpdated() bool {
	pods, err := p.PodLister.List(meta.ListOptions{LabelSelector: p.Label})

	if err != nil {
		p.Logger.Error("failed to list pods", err)
		return false
	}

	for _, pod := range pods.Items {
		if !isReady(pod) {
			return false
		}
	}
	return true
}

func isReady(pod apicore.Pod) bool {
	return pod.Status.ContainerStatuses[0].Ready
}
