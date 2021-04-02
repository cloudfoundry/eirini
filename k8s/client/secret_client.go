package client

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/patching"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Secret struct {
	clientSet kubernetes.Interface
}

func NewSecret(clientSet kubernetes.Interface) *Secret {
	return &Secret{clientSet: clientSet}
}

func (c *Secret) Get(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Secret) Create(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (c *Secret) Update(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
}

func (c *Secret) Delete(ctx context.Context, namespace string, name string) error {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Secret) SetOwner(ctx context.Context, secret *corev1.Secret, owner metav1.Object) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	if err := controllerutil.SetOwnerReference(owner, secret, scheme.Scheme); err != nil {
		return nil, errors.Wrap(err, "secret-client-set-owner-ref-failed")
	}

	patch := patching.NewSetOwner(secret.OwnerReferences[0])

	return c.clientSet.CoreV1().Secrets(secret.Namespace).Patch(ctx, secret.Name, patch.Type(), patch.GetPatchBytes(), metav1.PatchOptions{})
}
