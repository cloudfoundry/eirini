package stset

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
)

//counterfeiter:generate . PodDisruptionBudgetDeleter
//counterfeiter:generate . StatefulSetDeleter
//counterfeiter:generate . SecretsDeleter
//counterfeiter:generate . PodDeleter

type PodDisruptionBudgetDeleter interface {
	Delete(ctx context.Context, namespace string, name string) error
}

type StatefulSetDeleter interface {
	Delete(ctx context.Context, namespace string, name string) error
}

type SecretsDeleter interface {
	Delete(ctx context.Context, namespace string, name string) error
}

type PodDeleter interface {
	Delete(ctx context.Context, namespace, name string) error
}

type Stopper struct {
	logger              lager.Logger
	statefulSetDeleter  StatefulSetDeleter
	podDeleter          PodDeleter
	podDisruptionBudget PodDisruptionBudgetDeleter
	secretsDeleter      SecretsDeleter
	getStatefulSet      getStatefulSetFunc
}

func NewStopper(
	logger lager.Logger,
	statefulSetGetter StatefulSetByLRPIdentifierGetter,
	statefulSetDeleter StatefulSetDeleter,
	podDeleter PodDeleter,
	podDisruptionBudget PodDisruptionBudgetDeleter,
	secretsDeleter SecretsDeleter,
) Stopper {
	return Stopper{
		logger:              logger,
		statefulSetDeleter:  statefulSetDeleter,
		podDeleter:          podDeleter,
		podDisruptionBudget: podDisruptionBudget,
		secretsDeleter:      secretsDeleter,
		getStatefulSet:      newGetStatefulSetFunc(statefulSetGetter),
	}
}

func (s *Stopper) Stop(ctx context.Context, identifier opi.LRPIdentifier) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return s.stop(ctx, identifier)
	})

	return errors.Wrap(err, "failed to delete statefulset")
}

func (s *Stopper) stop(ctx context.Context, identifier opi.LRPIdentifier) error {
	logger := s.logger.Session("stop", lager.Data{"guid": identifier.GUID, "version": identifier.Version})
	statefulSet, err := s.getStatefulSet(ctx, identifier)

	if errors.Is(err, eirini.ErrNotFound) {
		logger.Debug("statefulset-does-not-exist")

		return nil
	}

	if err != nil {
		logger.Error("failed-to-get-statefulset", err)

		return err
	}

	err = s.podDisruptionBudget.Delete(ctx, statefulSet.Namespace, statefulSet.Name)
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error("failed-to-delete-disruption-budget", err)

		return errors.Wrap(err, "failed to delete pod disruption budget")
	}

	err = s.deletePrivateRegistrySecret(ctx, statefulSet)
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error("failed-to-delete-private-registry-secret", err)

		return err
	}

	if err := s.statefulSetDeleter.Delete(ctx, statefulSet.Namespace, statefulSet.Name); err != nil {
		logger.Error("failed-to-delete-statefulset", err)

		return errors.Wrap(err, "failed to delete statefulset")
	}

	return nil
}

func (s *Stopper) deletePrivateRegistrySecret(ctx context.Context, statefulSet *appsv1.StatefulSet) error {
	for _, secret := range statefulSet.Spec.Template.Spec.ImagePullSecrets {
		if secret.Name == privateRegistrySecretName(statefulSet.Name) {
			return s.secretsDeleter.Delete(ctx, statefulSet.Namespace, secret.Name)
		}
	}

	return nil
}

func (s *Stopper) StopInstance(ctx context.Context, identifier opi.LRPIdentifier, index uint) error {
	logger := s.logger.Session("stopInstance", lager.Data{"guid": identifier.GUID, "version": identifier.Version, "index": index})
	statefulset, err := s.getStatefulSet(ctx, identifier)

	if errors.Is(err, eirini.ErrNotFound) {
		logger.Debug("statefulset-does-not-exist")

		return nil
	}

	if err != nil {
		logger.Debug("failed-to-get-statefulset", lager.Data{"error": err.Error()})

		return err
	}

	if int32(index) >= *statefulset.Spec.Replicas {
		return eirini.ErrInvalidInstanceIndex
	}

	err = s.podDeleter.Delete(ctx, statefulset.Namespace, fmt.Sprintf("%s-%d", statefulset.Name, index))
	if k8serrors.IsNotFound(err) {
		logger.Debug("pod-does-not-exist")

		return nil
	}

	return errors.Wrap(err, "failed to delete pod")
}
