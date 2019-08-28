package waiter

import (
	"code.cloudfoundry.org/lager"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate counterfeiter . DeploymentLister
type DeploymentLister interface {
	List(metav1.ListOptions) (*appsv1.DeploymentList, error)
}

type Deployment struct {
	Deployments       DeploymentLister
	Logger            lager.Logger
	ListLabelSelector string
}

func (w Deployment) IsReady() bool {
	deploymentList, err := w.Deployments.List(metav1.ListOptions{LabelSelector: w.ListLabelSelector})
	if err != nil {
		w.Logger.Error("failed to list deployments", err)
		return false
	}

	for _, d := range deploymentList.Items {
		if !podsReady(d) || d.Generation != d.Status.ObservedGeneration {
			return false
		}
	}
	return true
}

func podsReady(d appsv1.Deployment) bool {
	desiredReplicas := *d.Spec.Replicas
	return d.Status.ReadyReplicas == desiredReplicas &&
		d.Status.UpdatedReplicas == desiredReplicas &&
		d.Status.AvailableReplicas == desiredReplicas &&
		d.Status.UnavailableReplicas == 0
}
