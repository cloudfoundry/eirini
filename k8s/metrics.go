package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . PodsGetter

type PodsGetter interface {
	GetAll(ctx context.Context) ([]corev1.Pod, error)
}
