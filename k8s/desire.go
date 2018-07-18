package k8s

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/launcher"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Desirer struct {
	KubeNamespace     string
	Client            *kubernetes.Clientset
	ingressManager    IngressManager
	deploymentManager DeploymentManager
	serviceManager    ServiceManager
}

func NewDesirer(client *kubernetes.Clientset, kubeNamespace string, ingressManager IngressManager, deploymentManager DeploymentManager, serviceManager ServiceManager) *Desirer {

	return &Desirer{
		KubeNamespace:     kubeNamespace,
		Client:            client,
		ingressManager:    ingressManager,
		deploymentManager: deploymentManager,
		serviceManager:    serviceManager,
	}
}

func (d *Desirer) Desire(ctx context.Context, lrps []opi.LRP) error {
	deployments, err := d.Client.AppsV1beta1().Deployments(d.KubeNamespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	dByName := make(map[string]struct{})
	for _, d := range deployments.Items {
		dByName[d.Name] = struct{}{}
	}

	for _, lrp := range lrps {
		if _, ok := dByName[lrp.Name]; ok {
			continue
		}

		if _, err := d.Client.AppsV1beta1().Deployments(d.KubeNamespace).Create(toDeployment(lrp)); err != nil {
			// fixme: this should be a multi-error and deferred
			return err
		}

		service, err := exposeDeployment(lrp, d.KubeNamespace)
		if err != nil {
			return err
		}

		if _, err = d.Client.CoreV1().Services(d.KubeNamespace).Create(service); err != nil {
			return err
		}

		if err = d.ingressManager.UpdateIngress(d.KubeNamespace, lrp); err != nil {
			return err
		}
	}

	return nil
}

func (d *Desirer) List(ctx context.Context) ([]opi.LRP, error) {
	return d.deploymentManager.ListLRPs(d.KubeNamespace)
}

func (d *Desirer) Get(ctx context.Context, name string) (*opi.LRP, error) {
	deployment, err := d.Client.AppsV1beta1().Deployments(d.KubeNamespace).Get(name, av1.GetOptions{})
	if err != nil {
		return nil, err
	}

	containers := deployment.Spec.Template.Spec.Containers
	assertSingleContainer(containers)
	container := containers[0]

	metadata := map[string]string{
		cf.LastUpdated: deployment.Annotations[cf.LastUpdated],
	}

	return &opi.LRP{
		Name:            deployment.Name,
		Image:           container.Image,
		Command:         container.Command,
		Env:             toMap(container.Env),
		TargetInstances: int(*deployment.Spec.Replicas),
		Metadata:        metadata,
	}, nil
}

func (d *Desirer) Update(ctx context.Context, updated opi.LRP) error {
	deployment, err := d.Client.AppsV1beta1().Deployments(d.KubeNamespace).Get(updated.Name, av1.GetOptions{})
	if err != nil {
		return err
	}

	count := int32(updated.TargetInstances)
	deployment.Spec.Replicas = &count
	deployment.Annotations[cf.LastUpdated] = updated.Metadata[cf.LastUpdated]

	_, err = d.Client.AppsV1beta1().Deployments(d.KubeNamespace).Update(deployment)
	return err
}

func (d *Desirer) Stop(ctx context.Context, name string) error {
	if err := d.deploymentManager.Delete(name, d.KubeNamespace); err != nil {
		return err
	}
	if err := d.serviceManager.Delete(name, d.KubeNamespace); err != nil {
		return err
	}

	return nil
}

func toMap(envVars []v1.EnvVar) map[string]string {
	result := make(map[string]string)
	for _, env := range envVars {
		result[env.Name] = env.Value
	}
	return result
}

func toDeployment(lrp opi.LRP) *v1beta1.Deployment {
	environment := launcher.SetupEnv(lrp.Command[0])
	deployment := &v1beta1.Deployment{
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32ptr(lrp.TargetInstances),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: asMultipleContainers(v1.Container{
						Name:  "web",
						Image: lrp.Image,
						Command: []string{
							launcher.Launch,
						},
						Env: mapToEnvVar(mergeMaps(lrp.Env, environment)),
						Ports: []v1.ContainerPort{
							v1.ContainerPort{
								Name:          "expose",
								ContainerPort: 8080,
							},
						},
					}),
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

// The Kubernetes API expects multiple containers but we only ever need one.
func asMultipleContainers(container v1.Container) []v1.Container {
	return []v1.Container{container}
}

// Enforce our assumption that there's only ever exactly one container holding the app.
func assertSingleContainer(containers []v1.Container) {
	if len(containers) != 1 {
		message := fmt.Sprintf("Unexpectedly, container count is not 1 but %d.", len(containers))
		panic(errors.New(message))
	}
}

func exposeDeployment(lrp opi.LRP, namespace string) (*v1.Service, error) {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:     "service",
					Port:     8080,
					Protocol: v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"name": lrp.Name,
			},
			SessionAffinity: "None",
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{},
		},
	}

	service.APIVersion = "v1"
	service.Kind = "Service"
	service.Name = eirini.GetInternalServiceName(lrp.Name)
	service.Namespace = namespace
	service.Labels = map[string]string{
		"eirini": "eirini",
		"name":   lrp.Name,
	}

	service.Annotations = map[string]string{
		"routes": lrp.Metadata[cf.VcapAppUris],
	}

	return service, nil
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func int32ptr(i int) *int32 {
	u := int32(i)
	return &u
}

func int64ptr(i int) *int64 {
	u := int64(i)
	return &u
}
