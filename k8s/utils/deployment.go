package utils

import (
	"code.cloudfoundry.org/lager"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate counterfeiter . DeploymentLister
type DeploymentLister interface {
	List(metav1.ListOptions) (*appsv1.DeploymentList, error)
}

func IsReady(lister DeploymentLister, logger lager.Logger, labelSelector string) bool {
	deploymentList, err := lister.List(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		logger.Error("failed to list deployments", err)
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
