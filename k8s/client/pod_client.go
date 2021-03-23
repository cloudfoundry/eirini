package client

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/patching"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Pod struct {
	clientSet          kubernetes.Interface
	workloadsNamespace string
}

func NewPod(clientSet kubernetes.Interface, workloadsNamespace string) *Pod {
	return &Pod{
		clientSet:          clientSet,
		workloadsNamespace: workloadsNamespace,
	}
}

func (c *Pod) GetAll(ctx context.Context) ([]corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	podList, err := c.clientSet.CoreV1().Pods(c.workloadsNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s in (%s,%s)",
			stset.LabelSourceType, "APP", "TASK",
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	return podList.Items, nil
}

func (c *Pod) GetByLRPIdentifier(ctx context.Context, id opi.LRPIdentifier) ([]corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	podList, err := c.clientSet.CoreV1().Pods(c.workloadsNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s=%s,%s=%s",
			stset.LabelGUID, id.GUID,
			stset.LabelVersion, id.Version,
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods by lrp identifier")
	}

	return podList.Items, nil
}

func (c *Pod) Delete(ctx context.Context, namespace, name string) error {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Pod) SetAnnotation(ctx context.Context, pod *corev1.Pod, key, value string) (*corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	annotation := patching.NewAnnotation(key, value)

	return c.clientSet.CoreV1().Pods(pod.Namespace).Patch(
		ctx,
		pod.Name,
		annotation.Type(),
		annotation.GetPatchBytes(),
		metav1.PatchOptions{},
	)
}

func (c *Pod) SetAndTestAnnotation(ctx context.Context, pod *corev1.Pod, key, value string, oldValue *string) (*corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	annotation := patching.NewTestingAnnotation(key, value, oldValue)

	return c.clientSet.CoreV1().Pods(pod.Namespace).Patch(
		ctx,
		pod.Name,
		annotation.Type(),
		annotation.GetPatchBytes(),
		metav1.PatchOptions{},
	)
}
