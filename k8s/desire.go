package k8s

import (
	"context"

	"github.com/julz/cube/opi"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Desirer struct {
	Client *kubernetes.Clientset
}

func (d *Desirer) Desire(ctx context.Context, lrps []opi.LRP) error {
	deployments, err := d.Client.AppsV1beta1().Deployments("default").List(av1.ListOptions{})
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

		if _, err := d.Client.AppsV1beta1().Deployments("default").Create(toDeployment(lrp)); err != nil {
			// fixme: this should be a multi-error and deferred
			return err
		}
	}

	return nil
}

func toDeployment(lrp opi.LRP) *v1beta1.Deployment {
	deployment := &v1beta1.Deployment{
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32ptr(lrp.TargetInstances),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:  "web",
						Image: lrp.Image,
					}},
				},
			},
		},
	}

	deployment.Name = lrp.Name
	deployment.Spec.Template.Labels = map[string]string{
		"name": lrp.Name,
	}

	deployment.Labels = map[string]string{
		"cube": "cube",
		"name": lrp.Name,
	}

	return deployment
}

func int32ptr(i int) *int32 {
	u := int32(i)
	return &u
}

func int64ptr(i int) *int64 {
	u := int64(i)
	return &u
}
