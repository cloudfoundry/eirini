package stset

import (
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//counterfeiter:generate . PodDisruptionBudgetCreator

type PodDisruptionBudgetCreator interface {
	Create(namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error)
}

type createPodDisruptionBudgetFunc func(namespace, statefulSetName string, lrp *opi.LRP) error

func newCreatePodDisruptionBudgetFunc(pdbCreator PodDisruptionBudgetCreator) createPodDisruptionBudgetFunc {
	return func(namespace, statefulSetName string, lrp *opi.LRP) error {
		if lrp.TargetInstances > 1 {
			minAvailable := intstr.FromInt(PdbMinAvailableInstances)
			_, err := pdbCreator.Create(namespace, &v1beta1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name: statefulSetName,
				},
				Spec: v1beta1.PodDisruptionBudgetSpec{
					MinAvailable: &minAvailable,
					Selector:     statefulSetLabelSelector(lrp),
				},
			})

			return errors.Wrap(err, "failed to create pod distruption budget")
		}

		return nil
	}
}
