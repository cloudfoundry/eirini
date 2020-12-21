package stset

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
)

//counterfeiter:generate . StatefulSetUpdater

type StatefulSetUpdater interface {
	Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
}

type Updater struct {
	logger                     lager.Logger
	statefulSetUpdater         StatefulSetUpdater
	podDisruptionBudgetDeleter PodDisruptionBudgetDeleter
	podDisruptionBudgetCreator PodDisruptionBudgetCreator
	getStatefulSet             getStatefulSetFunc
	createPodDisruptionBudget  createPodDisruptionBudgetFunc
}

func NewUpdater(
	logger lager.Logger,
	statefulSetGetter StatefulSetByLRPIdentifierGetter,
	statefulSetUpdater StatefulSetUpdater,
	podDisruptionBudgetDeleter PodDisruptionBudgetDeleter,
	podDisruptionBudgetCreator PodDisruptionBudgetCreator,
) Updater {
	return Updater{
		logger:                     logger,
		statefulSetUpdater:         statefulSetUpdater,
		podDisruptionBudgetDeleter: podDisruptionBudgetDeleter,
		podDisruptionBudgetCreator: podDisruptionBudgetCreator,
		getStatefulSet:             newGetStatefulSetFunc(statefulSetGetter),
		createPodDisruptionBudget:  newCreatePodDisruptionBudgetFunc(podDisruptionBudgetCreator),
	}
}

func (u *Updater) Update(lrp *opi.LRP) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return u.update(lrp)
	})

	return errors.Wrap(err, "failed to update statefulset")
}

func (u *Updater) update(lrp *opi.LRP) error {
	logger := u.logger.Session("update", lager.Data{"guid": lrp.GUID, "version": lrp.Version})

	statefulSet, err := u.getStatefulSet(opi.LRPIdentifier{GUID: lrp.GUID, Version: lrp.Version})
	if err != nil {
		logger.Error("failed-to-get-statefulset", err)

		return err
	}

	updatedStatefulSet, err := u.getUpdatedStatefulSetObj(statefulSet,
		lrp.AppURIs,
		lrp.TargetInstances,
		lrp.LastUpdated,
		lrp.Image,
	)
	if err != nil {
		logger.Error("failed-to-get-updated-statefulset", err)

		return err
	}

	_, err = u.statefulSetUpdater.Update(updatedStatefulSet.Namespace, updatedStatefulSet)
	if err != nil {
		logger.Error("failed-to-update-statefulset", err, lager.Data{"namespace": statefulSet.Namespace})

		return errors.Wrap(err, "failed to update statefulset")
	}

	return u.handlePodDisruptionBudget(logger,
		statefulSet.Namespace,
		statefulSet.Name,
		lrp,
	)
}

func (u *Updater) getUpdatedStatefulSetObj(sts *appsv1.StatefulSet, routes []opi.Route, instances int, lastUpdated, image string) (*appsv1.StatefulSet, error) {
	updatedSts := sts.DeepCopy()

	uris, err := json.Marshal(routes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal routes")
	}

	count := int32(instances)
	updatedSts.Spec.Replicas = &count
	updatedSts.Annotations[AnnotationLastUpdated] = lastUpdated
	updatedSts.Annotations[AnnotationRegisteredRoutes] = string(uris)

	if image != "" {
		for i, container := range updatedSts.Spec.Template.Spec.Containers {
			if container.Name == OPIContainerName {
				updatedSts.Spec.Template.Spec.Containers[i].Image = image
			}
		}
	}

	return updatedSts, nil
}

func (u *Updater) handlePodDisruptionBudget(logger lager.Logger, namespace, name string, lrp *opi.LRP) error {
	if lrp.TargetInstances <= 1 {
		err := u.podDisruptionBudgetDeleter.Delete(namespace, name)
		if err != nil && !k8serrors.IsNotFound(err) {
			logger.Error("failed-to-delete-disruption-budget", err, lager.Data{"namespace": namespace})

			return errors.Wrap(err, "failed to delete pod disruption budget")
		}

		return nil
	}

	err := u.createPodDisruptionBudget(namespace, name, lrp)

	if err != nil && !k8serrors.IsAlreadyExists(err) {
		logger.Error("failed-to-create-disruption-budget", err, lager.Data{"namespace": namespace})

		return err
	}

	return nil
}
