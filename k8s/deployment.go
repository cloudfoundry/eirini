package k8s

import (
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ev1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
)

type deploymentManager struct {
	client    kubernetes.Interface
	namespace string
}

func NewDeploymentManager(namespace string, client kubernetes.Interface) InstanceManager {
	return &deploymentManager{
		namespace: namespace,
		client:    client,
	}
}

func (m *deploymentManager) deployments() ev1.DeploymentInterface {
	return m.client.AppsV1beta1().Deployments(m.namespace)
}

func (m *deploymentManager) Create(lrp *opi.LRP) error {
	_, err := m.deployments().Create(toDeployment(lrp))
	return err
}

func (m *deploymentManager) List() ([]*opi.LRP, error) {
	deployments, err := m.deployments().List(av1.ListOptions{})
	if err != nil {
		return nil, err
	}

	lrps := deploymentsToLRPs(deployments)

	return lrps, nil
}

func (m *deploymentManager) Delete(appName string) error {
	backgroundPropagation := av1.DeletePropagationBackground
	return m.deployments().Delete(appName, &av1.DeleteOptions{PropagationPolicy: &backgroundPropagation})
}

func (m *deploymentManager) Update(lrp *opi.LRP) error {
	return nil
}

func (m *deploymentManager) Exists(appName string) (bool, error) {
	return true, nil
}

func (m *deploymentManager) Get(appName string) (*opi.LRP, error) {
	return nil, nil
}

func deploymentsToLRPs(deployments *v1beta1.DeploymentList) []*opi.LRP {
	lrps := []*opi.LRP{}
	for _, d := range deployments.Items {
		lrp := &opi.LRP{
			Name:    d.Name,
			Command: d.Spec.Template.Spec.Containers[0].Command,
			Image:   d.Spec.Template.Spec.Containers[0].Image,
			Metadata: map[string]string{
				cf.ProcessGUID: d.Annotations[cf.ProcessGUID],
				cf.LastUpdated: d.Annotations[cf.LastUpdated],
			},
		}
		lrps = append(lrps, lrp)
	}
	return lrps
}

func toDeployment(lrp *opi.LRP) *v1beta1.Deployment {
	deployment := &v1beta1.Deployment{
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32ptr(lrp.TargetInstances),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     MapToEnvVar(lrp.Env),
							Ports: []v1.ContainerPort{
								v1.ContainerPort{
									Name:          "expose",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	deployment.Name = lrp.Name
	deployment.Spec.Template.Labels = map[string]string{
		"name": lrp.Name,
	}

	deployment.Labels = map[string]string{
		"eirini": "eirini",
		"name":   lrp.Name,
	}

	deployment.Annotations = lrp.Metadata

	return deployment
}
