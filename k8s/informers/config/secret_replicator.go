package config

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type secretReplicator struct {
	secretsClient SecretsClient
}

//counterfeiter:generate . SecretsClient
type SecretsClient interface {
	Get(namespace, name string) (*corev1.Secret, error)
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	Update(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
}

func NewSecretReplicator(secretsClient SecretsClient) SecretReplicator {
	return &secretReplicator{
		secretsClient: secretsClient,
	}
}

func (r *secretReplicator) ReplicateSecret(srcNamespace, srcSecretName, dstNamespace, dstSecretName string) error {
	srcSecret, err := r.secretsClient.Get(srcNamespace, srcSecretName)
	if err != nil {
		return errors.Wrapf(err, "replicate secret: failed to get source secret: %s/%s", srcNamespace, srcSecretName)
	}

	_, err = r.secretsClient.Create(dstNamespace, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dstSecretName,
			Namespace: dstNamespace,
		},
		Type: srcSecret.Type,
		Data: srcSecret.Data,
	})
	if k8s_errors.IsAlreadyExists(err) {
		dstSecret, getErr := r.secretsClient.Get(dstNamespace, dstSecretName)
		if getErr != nil {
			return errors.Wrapf(getErr, "replicate secret: failed to get existing destination secret: %s/%s", dstNamespace, dstSecretName)
		}
		dstSecret.Data = srcSecret.Data
		if _, updateErr := r.secretsClient.Update(dstNamespace, dstSecret); updateErr != nil {
			return errors.Wrapf(updateErr, "replicate secret: failed to update existing destination secret: %s/%s", dstNamespace, dstSecretName)
		}
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "replicate secret: failed to create new destination secret: %s/%s", dstNamespace, dstSecretName)
	}
	return nil
}
