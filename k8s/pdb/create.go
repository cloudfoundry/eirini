package pdb

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	"k8s.io/api/policy/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//counterfeiter:generate . K8sClient

type K8sClient interface {
	Create(ctx context.Context, namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error)
	Delete(ctx context.Context, namespace string, name string) error
}

const PdbMinAvailableInstances = 1

type CreatorDeleter struct {
	pdbClient K8sClient
}

func NewCreatorDeleter(pdbClient K8sClient) *CreatorDeleter {
	return &CreatorDeleter{
		pdbClient: pdbClient,
	}
}

func (c *CreatorDeleter) Update(ctx context.Context, namespace, name string, lrp *opi.LRP) error {
	minAvailable := intstr.FromInt(PdbMinAvailableInstances)

	if lrp.TargetInstances > 1 {
		_, err := c.pdbClient.Create(ctx, namespace, &v1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					stset.LabelGUID:    lrp.GUID,
					stset.LabelVersion: lrp.Version,
				},
			},
			Spec: v1beta1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
				Selector:     stset.StatefulSetLabelSelector(lrp),
			},
		})

		if k8serrors.IsAlreadyExists(err) {
			return nil
		}

		return errors.Wrap(err, "failed to create pod distruption budget")
	}

	err := c.Delete(ctx, namespace, name)

	if k8serrors.IsNotFound(err) {
		return nil
	}

	return errors.Wrap(err, "failed to delete pod distruption budget")
}

func (c *CreatorDeleter) Delete(ctx context.Context, namespace, name string) error {
	return c.pdbClient.Delete(ctx, namespace, name)
}
