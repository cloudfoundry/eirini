package pdb

import (
	"context"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//counterfeiter:generate . K8sClient

type K8sClient interface {
	Create(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) (*policyv1.PodDisruptionBudget, error)
	Delete(ctx context.Context, namespace string, name string) error
}

const PdbMinAvailableInstances = "50%"

type Updater struct {
	pdbClient K8sClient
}

func NewUpdater(pdbClient K8sClient) *Updater {
	return &Updater{
		pdbClient: pdbClient,
	}
}

func (c *Updater) Update(ctx context.Context, statefulSet *appsv1.StatefulSet, lrp *api.LRP) error {
	if lrp.TargetInstances > 1 {
		return c.createPDB(ctx, statefulSet, lrp)
	}

	return c.deletePDB(ctx, statefulSet)
}

func (c *Updater) createPDB(ctx context.Context, statefulSet *appsv1.StatefulSet, lrp *api.LRP) error {
	minAvailable := intstr.FromString(PdbMinAvailableInstances)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSet.Name,
			Namespace: statefulSet.Namespace,
			Labels: map[string]string{
				stset.LabelGUID:    lrp.GUID,
				stset.LabelVersion: lrp.Version,
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector:     stset.StatefulSetLabelSelector(lrp),
		},
	}

	if err := controllerutil.SetOwnerReference(statefulSet, pdb, scheme.Scheme); err != nil {
		return errors.Wrap(err, "pdb-updated-failed-to-set-owner-ref")
	}

	_, err := c.pdbClient.Create(ctx, statefulSet.Namespace, pdb)

	if k8serrors.IsAlreadyExists(err) {
		return nil
	}

	return errors.Wrap(err, "failed to create pod distruption budget")
}

func (c *Updater) deletePDB(ctx context.Context, statefulSet *appsv1.StatefulSet) error {
	err := c.pdbClient.Delete(ctx, statefulSet.Namespace, statefulSet.Name)

	if k8serrors.IsNotFound(err) {
		return nil
	}

	return errors.Wrap(err, "failed to delete pod distruption budget")
}
