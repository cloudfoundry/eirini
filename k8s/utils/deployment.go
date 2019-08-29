package utils

import (
	"code.cloudfoundry.org/lager"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate counterfeiter . DeploymentClient
type DeploymentClient interface {
	Get(name string, options metav1.GetOptions) (*appsv1.Deployment, error)
}

func IsReady(client DeploymentClient, logger lager.Logger, deploymentName string) bool {
	deployment, err := client.Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		logger.Error("failed to list deployments", err)
		return false
	}

	if !podsReady(*deployment) || deployment.Generation != deployment.Status.ObservedGeneration {
		return false
	}

	debugData := map[string]interface{}{
		"deploymentName":       deployment.Name,
		"deploymentStatus":     deployment.Status,
		"deploymentGeneration": deployment.Generation,
	}
	logger.Debug("Deployment is updated and ready", debugData)
	return true
}

func podsReady(d appsv1.Deployment) bool {
	desiredReplicas := *d.Spec.Replicas
	return d.Status.ReadyReplicas == desiredReplicas &&
		d.Status.UpdatedReplicas == desiredReplicas &&
		d.Status.AvailableReplicas == desiredReplicas &&
		d.Status.UnavailableReplicas == 0
}
