package client

import (
	"context"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PodDisruptionBudget struct {
	clientSet kubernetes.Interface
}

func NewPodDisruptionBudget(clientSet kubernetes.Interface) *PodDisruptionBudget {
	return &PodDisruptionBudget{clientSet: clientSet}
}

func (c *PodDisruptionBudget) Create(ctx context.Context, namespace string, podDisruptionBudget *policyv1beta1.PodDisruptionBudget) (*policyv1beta1.PodDisruptionBudget, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Create(ctx, podDisruptionBudget, metav1.CreateOptions{})
}

func (c *PodDisruptionBudget) Delete(ctx context.Context, namespace string, name string) error {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
