package client

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Event struct {
	clientSet kubernetes.Interface
}

func NewEvent(clientSet kubernetes.Interface) *Event {
	return &Event{
		clientSet: clientSet,
	}
}

func (c *Event) GetByPod(ctx context.Context, pod corev1.Pod) ([]corev1.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	eventList, err := c.clientSet.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf(
			"involvedObject.namespace=%s,involvedObject.uid=%s,involvedObject.name=%s",
			pod.Namespace,
			string(pod.UID),
			pod.Name,
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pod events")
	}

	return eventList.Items, nil
}

func (c *Event) GetByInstanceAndReason(ctx context.Context, namespace string, ownerRef metav1.OwnerReference, instanceIndex int, reason string) (*corev1.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	fieldSelector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s,involvedObject.namespace=%s,reason=%s",
		ownerRef.Kind,
		ownerRef.Name,
		namespace,
		reason,
	)
	labelSelector := fmt.Sprintf("cloudfoundry.org/instance_index=%d", instanceIndex)

	kubeEvents, err := c.clientSet.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list events")
	}

	if len(kubeEvents.Items) == 1 {
		return &kubeEvents.Items[0], nil
	}

	return nil, nil
}

func (c *Event) Create(ctx context.Context, namespace string, event *corev1.Event) (*corev1.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
}

func (c *Event) Update(ctx context.Context, namespace string, event *corev1.Event) (*corev1.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.CoreV1().Events(namespace).Update(ctx, event, metav1.UpdateOptions{})
}
