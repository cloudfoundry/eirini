package client

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/patching"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type StatefulSet struct {
	clientSet          kubernetes.Interface
	workloadsNamespace string
}

func NewStatefulSet(clientSet kubernetes.Interface, workloadsNamespace string) *StatefulSet {
	return &StatefulSet{
		clientSet:          clientSet,
		workloadsNamespace: workloadsNamespace,
	}
}

func (c *StatefulSet) Create(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.AppsV1().StatefulSets(namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
}

func (c *StatefulSet) Get(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *StatefulSet) GetBySourceType(ctx context.Context, sourceType string) ([]appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	statefulSetList, err := c.clientSet.AppsV1().StatefulSets(c.workloadsNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", stset.LabelSourceType, sourceType),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets by resource type")
	}

	return statefulSetList.Items, nil
}

func (c *StatefulSet) GetByLRPIdentifier(ctx context.Context, id opi.LRPIdentifier) ([]appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	statefulSetList, err := c.clientSet.AppsV1().StatefulSets(c.workloadsNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s=%s,%s=%s",
			stset.LabelGUID, id.GUID,
			stset.LabelVersion, id.Version,
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets by lrp identifier")
	}

	return statefulSetList.Items, nil
}

func (c *StatefulSet) Update(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.AppsV1().StatefulSets(namespace).Update(ctx, statefulSet, metav1.UpdateOptions{})
}

func (c *StatefulSet) SetAnnotation(ctx context.Context, statefulSet *appsv1.StatefulSet, key, value string) (*appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	annotation := patching.NewAnnotation(key, value)

	return c.clientSet.AppsV1().StatefulSets(statefulSet.Namespace).Patch(
		ctx,
		statefulSet.Name,
		annotation.Type(),
		annotation.GetPatchBytes(),
		metav1.PatchOptions{},
	)
}

func (c *StatefulSet) SetCPURequest(ctx context.Context, statefulSet *appsv1.StatefulSet, cpuRequest *resource.Quantity) (*appsv1.StatefulSet, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	cpuRequestPatch := patching.NewCPURequestPatch(statefulSet, cpuRequest)

	return c.clientSet.AppsV1().StatefulSets(statefulSet.Namespace).Patch(
		ctx,
		statefulSet.Name,
		cpuRequestPatch.Type(),
		cpuRequestPatch.GetPatchBytes(),
		metav1.PatchOptions{},
	)
}

func (c *StatefulSet) Delete(ctx context.Context, namespace string, name string) error {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	backgroundPropagation := metav1.DeletePropagationBackground

	return c.clientSet.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
}
