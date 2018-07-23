package k8s

import (
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/apps/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ev1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
)

//go:generate counterfeiter . DeploymentManager
type DeploymentManager interface {
	ListLRPs(namespace string) ([]opi.LRP, error)
	Delete(appName, namespace string) error
}

type deploymentManager struct {
	client   kubernetes.Interface
	endpoint string
}

func NewDeploymentManager(client kubernetes.Interface) DeploymentManager {
	return &deploymentManager{
		client: client,
	}
}

func (m *deploymentManager) ListLRPs(namespace string) ([]opi.LRP, error) {
	deployments, err := m.deployments(namespace).List(av1.ListOptions{})
	if err != nil {
		return nil, err
	}

	lrps := toLRPs(deployments)

	return lrps, nil
}

func (m *deploymentManager) Delete(appName, namespace string) error {
	return m.deployments(namespace).Delete(appName, &av1.DeleteOptions{})
}

func (m *deploymentManager) deployments(namespace string) ev1.DeploymentInterface {
	return m.client.AppsV1beta1().Deployments(namespace)
}

func toLRPs(deployments *v1beta1.DeploymentList) []opi.LRP {
	lrps := []opi.LRP{}
	for _, d := range deployments.Items {
		lrp := opi.LRP{
			Metadata: map[string]string{
				cf.ProcessGUID: d.Annotations[cf.ProcessGUID],
				cf.LastUpdated: d.Annotations[cf.LastUpdated],
			},
		}
		lrps = append(lrps, lrp)
	}
	return lrps
}
