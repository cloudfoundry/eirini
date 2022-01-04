package client

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/patching"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PodDisruptionBudget struct {
	clientSet kubernetes.Interface
}

func NewPodDisruptionBudget(clientSet kubernetes.Interface) *PodDisruptionBudget {
	return &PodDisruptionBudget{clientSet: clientSet}
}

func (c *PodDisruptionBudget) Get(ctx context.Context, namespace, name string) (*policyv1beta1.PodDisruptionBudget, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Get(ctx, name, metav1.GetOptions{})
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

func (c *PodDisruptionBudget) SetOwner(ctx context.Context, pdb *policyv1beta1.PodDisruptionBudget, owner *appsv1.StatefulSet) (*policyv1beta1.PodDisruptionBudget, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	if err := controllerutil.SetOwnerReference(owner, pdb, scheme.Scheme); err != nil {
		return nil, errors.Wrap(err, "pdb-client-set-owner-ref-failed")
	}

	patch := patching.NewSetOwner(pdb.OwnerReferences[0])

	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(pdb.Namespace).Patch(ctx, pdb.Name, patch.Type(), patch.GetPatchBytes(), metav1.PatchOptions{})
}
