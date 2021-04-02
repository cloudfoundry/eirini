package migrations

import (
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type AdoptJobRegistrySecret struct {
	secretsClient SecretsClient
}

func NewAdoptJobRegistrySecret(secretsClient SecretsClient) AdoptJobRegistrySecret {
	return AdoptJobRegistrySecret{
		secretsClient: secretsClient,
	}
}

func (m AdoptJobRegistrySecret) Apply(ctx context.Context, obj runtime.Object) error {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return fmt.Errorf("expected *batchv1.Job, got: %T", obj)
	}

	imagePullSecrets := job.Spec.Template.Spec.ImagePullSecrets
	for _, secret := range imagePullSecrets {
		prefix := dockerImagePullSecretNamePrefix(
			job.Annotations[jobs.AnnotationAppName],
			job.Annotations[jobs.AnnotationSpaceName],
			job.Labels[jobs.LabelGUID],
		)

		if !strings.HasPrefix(secret.Name, prefix) {
			continue
		}

		privateRegistrySecret, err := m.secretsClient.Get(ctx, job.Namespace, secret.Name)
		if err != nil {
			return errors.Wrapf(err, "failed to get secret %s", secret.Name)
		}

		_, err = m.secretsClient.SetOwner(ctx, privateRegistrySecret, job)
		if err != nil {
			return errors.Wrapf(err, "failed to set ownership on secret %s", secret.Name)
		}
	}

	return nil
}

func (m AdoptJobRegistrySecret) SequenceID() int {
	return AdoptJobSecretSequenceID
}

func dockerImagePullSecretNamePrefix(appName, spaceName, taskGUID string) string {
	secretNamePrefix := fmt.Sprintf("%s-%s", appName, spaceName)

	return fmt.Sprintf("%s-registry-secret-", utils.SanitizeName(secretNamePrefix, taskGUID))
}

func (m AdoptJobRegistrySecret) AppliesTo() ObjectType {
	return JobObjectType
}
