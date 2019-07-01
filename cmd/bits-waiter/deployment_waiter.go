package main

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate counterfeiter . DeploymentLister
type DeploymentLister interface {
	List(metav1.ListOptions) (*appsv1.DeploymentList, error)
}

type DeploymentWaiter struct {
	Deployments       DeploymentLister
	Timeout           time.Duration
	Logger            lager.Logger
	ExpectedPodLabels map[string]string
	ListLabelSelector string
}

func (w DeploymentWaiter) Wait() error {
	ready := make(chan interface{}, 1)
	defer close(ready)

	stop := make(chan interface{}, 1)
	defer close(stop)

	t := time.NewTimer(w.Timeout)
	if w.Timeout < 0 {
		return errors.New("provided timeout is not valid")
	}
	go w.poll(ready, stop)
	select {
	case <-ready:
		stop <- nil
		return nil
	case <-t.C:
		stop <- nil
		return fmt.Errorf("timed out after %s", w.Timeout.String())
	}
}

func (w DeploymentWaiter) poll(ready chan<- interface{}, stop <-chan interface{}) {
	for {
		select {
		case <-stop:
			return
		default:
			if w.deploymentsUpdated() {
				ready <- nil
				return
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (w DeploymentWaiter) deploymentsUpdated() bool {
	deploymentList, err := w.Deployments.List(metav1.ListOptions{LabelSelector: w.ListLabelSelector})
	if err != nil {
		w.Logger.Error("failed to list deployments", err)
		return false
	}

	for _, d := range deploymentList.Items {
		if !podsReady(d) || !w.expectedPodLabelsSet(d) || d.Generation != d.Status.ObservedGeneration {
			return false
		}
	}
	return true
}

func (w DeploymentWaiter) expectedPodLabelsSet(deployment appsv1.Deployment) bool {
	for key, expectedValue := range w.ExpectedPodLabels {
		actualValue, ok := deployment.Spec.Template.Labels[key]
		if !ok || expectedValue != actualValue {
			return false
		}
	}

	return true
}

func podsReady(d appsv1.Deployment) bool {
	desiredReplicas := *d.Spec.Replicas
	return d.Status.ReadyReplicas == desiredReplicas &&
		d.Status.UpdatedReplicas == desiredReplicas &&
		d.Status.AvailableReplicas == desiredReplicas
}
