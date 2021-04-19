package stset

import (
	"context"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//counterfeiter:generate . SecretsClient
//counterfeiter:generate . StatefulSetCreator
//counterfeiter:generate . LRPToStatefulSetConverter
//counterfeiter:generate . PodDisruptionBudgetUpdater

type LRPToStatefulSetConverter interface {
	Convert(statefulSetName string, lrp *api.LRP, privateRegistrySecret *corev1.Secret) (*appsv1.Deployment, error)
}

type SecretsClient interface {
	Create(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	SetOwner(ctx context.Context, secret *corev1.Secret, owner metav1.Object) (*corev1.Secret, error)
	Delete(ctx context.Context, namespace string, name string) error
}

type StatefulSetCreator interface {
	Create(ctx context.Context, namespace string, statefulSet *appsv1.Deployment) (*appsv1.Deployment, error)
}

type PodDisruptionBudgetUpdater interface {
	Update(ctx context.Context, stset *appsv1.Deployment, lrp *api.LRP) error
}

type Desirer struct {
	logger                     lager.Logger
	secrets                    SecretsClient
	statefulSets               StatefulSetCreator
	lrpToStatefulSetConverter  LRPToStatefulSetConverter
	podDisruptionBudgetCreator PodDisruptionBudgetUpdater
}

func NewDesirer(
	logger lager.Logger,
	secrets SecretsClient,
	statefulSets StatefulSetCreator,
	lrpToStatefulSetConverter LRPToStatefulSetConverter,
	podDisruptionBudgetCreator PodDisruptionBudgetUpdater,
) Desirer {
	return Desirer{
		logger:                     logger,
		secrets:                    secrets,
		statefulSets:               statefulSets,
		lrpToStatefulSetConverter:  lrpToStatefulSetConverter,
		podDisruptionBudgetCreator: podDisruptionBudgetCreator,
	}
}

func (d *Desirer) Desire(ctx context.Context, namespace string, lrp *api.LRP, opts ...shared.Option) error {
	logger := d.logger.Session("desire", lager.Data{"guid": lrp.GUID, "version": lrp.Version, "namespace": namespace})

	statefulSetName, err := utils.GetStatefulsetName(lrp)
	if err != nil {
		return err
	}

	privateRegistrySecret, err := d.createRegistryCredsSecretIfRequired(ctx, namespace, lrp)
	if err != nil {
		return err
	}

	st, err := d.lrpToStatefulSetConverter.Convert(statefulSetName, lrp, privateRegistrySecret)
	if err != nil {
		return err
	}

	st.Namespace = namespace

	err = shared.ApplyOpts(st, opts...)
	if err != nil {
		return err
	}

	stSet, err := d.statefulSets.Create(ctx, namespace, st)
	if err != nil {
		var statusErr *k8serrors.StatusError
		if errors.As(err, &statusErr) && statusErr.Status().Reason == metav1.StatusReasonAlreadyExists {
			logger.Debug("statefulset-already-exists", lager.Data{"error": err.Error()})

			return nil
		}

		return d.cleanupAndError(ctx, errors.Wrap(err, "failed to create statefulset"), privateRegistrySecret)
	}

	if err := d.setSecretOwner(ctx, privateRegistrySecret, stSet); err != nil {
		logger.Error("failed-to-set-owner-to-the-registry-secret", err)

		return errors.Wrap(err, "failed to set owner to the registry secret")
	}

	if err := d.podDisruptionBudgetCreator.Update(ctx, stSet, lrp); err != nil {
		logger.Error("failed-to-create-pod-disruption-budget", err)

		return errors.Wrap(err, "failed to create pod disruption budget")
	}

	return nil
}

func (d *Desirer) setSecretOwner(ctx context.Context, privateRegistrySecret *corev1.Secret, stSet *appsv1.Deployment) error {
	if privateRegistrySecret == nil {
		return nil
	}

	_, err := d.secrets.SetOwner(ctx, privateRegistrySecret, stSet)

	return err
}

func (d *Desirer) createRegistryCredsSecretIfRequired(ctx context.Context, namespace string, lrp *api.LRP) (*corev1.Secret, error) {
	if lrp.PrivateRegistry == nil {
		return nil, nil
	}

	secret, err := generateRegistryCredsSecret(lrp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate private registry secret for statefulset")
	}

	secret, err = d.secrets.Create(ctx, namespace, secret)

	return secret, errors.Wrap(err, "failed to create private registry secret for statefulset")
}

func (d *Desirer) cleanupAndError(ctx context.Context, stsetCreationError error, privateRegistrySecret *corev1.Secret) error {
	resultError := multierror.Append(nil, stsetCreationError)

	if privateRegistrySecret != nil {
		err := d.secrets.Delete(ctx, privateRegistrySecret.Namespace, privateRegistrySecret.Name)
		if err != nil {
			resultError = multierror.Append(resultError, errors.Wrap(err, "failed to cleanup registry secret"))
		}
	}

	return resultError
}

func generateRegistryCredsSecret(lrp *api.LRP) (*corev1.Secret, error) {
	dockerConfig := dockerutils.NewDockerConfig(
		lrp.PrivateRegistry.Server,
		lrp.PrivateRegistry.Username,
		lrp.PrivateRegistry.Password,
	)

	dockerConfigJSON, err := dockerConfig.JSON()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate privete registry config")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: PrivateRegistrySecretGenerateName,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			dockerutils.DockerConfigKey: dockerConfigJSON,
		},
	}, nil
}
