package k8s

import (
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/apps/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate counterfeiter . DeploymentManager
type DeploymentManager interface {
	ListLRPs(namespace string) ([]opi.LRP, error)
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
	deployments, err := m.client.AppsV1beta1().Deployments(namespace).List(av1.ListOptions{})
	if err != nil {
		return nil, err
	}

	lrps := toLRPs(deployments)

	return lrps, nil
}

func toLRPs(deployments *v1beta1.DeploymentList) []opi.LRP {
	lrps := []opi.LRP{}
	for _, d := range deployments.Items {
		lrp := opi.LRP{
			Metadata: map[string]string{
				cf.ProcessGuid: d.Annotations[cf.ProcessGuid],
				cf.LastUpdated: d.Annotations[cf.LastUpdated],
			},
		}
		lrps = append(lrps, lrp)
	}
	return lrps
}
