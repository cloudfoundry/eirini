package migrations

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//counterfeiter:generate . SecretsClient

type SecretsClient interface {
	Get(ctx context.Context, namespace, name string) (*corev1.Secret, error)
	SetOwner(ctx context.Context, secret *corev1.Secret, owner *appsv1.StatefulSet) (*corev1.Secret, error)
}

type AdoptStatefulsetRegistrySecret struct {
	secretsClient SecretsClient
}

func NewAdoptStatefulsetRegistrySecret(secretsClient SecretsClient) AdoptStatefulsetRegistrySecret {
	return AdoptStatefulsetRegistrySecret{
		secretsClient: secretsClient,
	}
}

func (m AdoptStatefulsetRegistrySecret) Apply(ctx context.Context, obj runtime.Object) error {
	stSet, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return fmt.Errorf("expected *v1.StatefulSet, got: %T", obj)
	}

	imagePullSecrets := stSet.Spec.Template.Spec.ImagePullSecrets
	for _, secret := range imagePullSecrets {
		if secret.Name != registryCredentialsSecretName(stSet.Name) {
			continue
		}

		privateRegistrySecret, err := m.secretsClient.Get(ctx, stSet.Namespace, imagePullSecrets[1].Name)
		if err != nil {
			return errors.Wrapf(err, "failed to get secret %s", imagePullSecrets[1].Name)
		}

		_, err = m.secretsClient.SetOwner(ctx, privateRegistrySecret, stSet)
		if err != nil {
			return errors.Wrapf(err, "failed to set ownership on secret %s", imagePullSecrets[1].Name)
		}
	}

	return nil
}

func (m AdoptStatefulsetRegistrySecret) SequenceID() int {
	return AdoptStSetSecretSequenceID
}

func registryCredentialsSecretName(statefulSetName string) string {
	return fmt.Sprintf("%s-registry-credentials", statefulSetName)
}
