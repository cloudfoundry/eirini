package client

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/patching"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func (c *Pod) GetAll() ([]corev1.Pod, error) {
	podList, err := c.clientSet.CoreV1().Pods(c.workloadsNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s in (%s,%s,%s)",
			k8s.LabelSourceType, "STG", "APP", "TASK",
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	return podList.Items, nil
}

func (c *Pod) GetByLRPIdentifier(id opi.LRPIdentifier) ([]corev1.Pod, error) {
	podList, err := c.clientSet.CoreV1().Pods(c.workloadsNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s=%s,%s=%s",
			k8s.LabelGUID, id.GUID,
			k8s.LabelVersion, id.Version,
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods by lrp identifier")
	}

	return podList.Items, nil
}

func (c *Pod) Delete(namespace, name string) error {
	return c.clientSet.CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func (c *Pod) SetAnnotation(pod *corev1.Pod, key, value string) (*corev1.Pod, error) {
	patchBytes := patching.NewAnnotation(key, value).GetJSONPatchBytes()

	return c.clientSet.CoreV1().Pods(pod.Namespace).Patch(
		context.Background(),
		pod.Name,
		types.JSONPatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
}

type PodDisruptionBudget struct {
	clientSet kubernetes.Interface
}

func NewPodDisruptionBudget(clientSet kubernetes.Interface) *PodDisruptionBudget {
	return &PodDisruptionBudget{clientSet: clientSet}
}

func (c *PodDisruptionBudget) Create(namespace string, podDisruptionBudget *policyv1beta1.PodDisruptionBudget) (*policyv1beta1.PodDisruptionBudget, error) {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Create(context.Background(), podDisruptionBudget, metav1.CreateOptions{})
}

func (c *PodDisruptionBudget) Delete(namespace string, name string) error {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

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

func (c *StatefulSet) Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Create(context.Background(), statefulSet, metav1.CreateOptions{})
}

func (c *StatefulSet) Get(namespace, name string) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *StatefulSet) GetBySourceType(sourceType string) ([]appsv1.StatefulSet, error) {
	statefulSetList, err := c.clientSet.AppsV1().StatefulSets(c.workloadsNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", k8s.LabelSourceType, sourceType),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets by resource type")
	}

	return statefulSetList.Items, nil
}

func (c *StatefulSet) GetByLRPIdentifier(id opi.LRPIdentifier) ([]appsv1.StatefulSet, error) {
	statefulSetList, err := c.clientSet.AppsV1().StatefulSets(c.workloadsNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s=%s,%s=%s",
			k8s.LabelGUID, id.GUID,
			k8s.LabelVersion, id.Version,
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets by lrp identifier")
	}

	return statefulSetList.Items, nil
}

func (c *StatefulSet) Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Update(context.Background(), statefulSet, metav1.UpdateOptions{})
}

func (c *StatefulSet) Delete(namespace string, name string) error {
	backgroundPropagation := metav1.DeletePropagationBackground

	return c.clientSet.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
}

type Job struct {
	clientSet          kubernetes.Interface
	workloadsNamespace string
	jobType            string
	guidLabel          string
}

func NewJob(clientSet kubernetes.Interface, workloadsNamespace string) *Job {
	return &Job{
		clientSet:          clientSet,
		workloadsNamespace: workloadsNamespace,
		jobType:            "TASK",
		guidLabel:          k8s.LabelGUID,
	}
}

func NewStagingJob(clientSet kubernetes.Interface, workloadsNamespace string) *Job {
	return &Job{
		clientSet:          clientSet,
		workloadsNamespace: workloadsNamespace,
		jobType:            "STG",
		guidLabel:          k8s.LabelStagingGUID,
	}
}

func (c *Job) Create(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
}

func (c *Job) Delete(namespace string, name string) error {
	backgroundPropagation := metav1.DeletePropagationBackground
	deleteOpts := metav1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	}

	return c.clientSet.BatchV1().Jobs(namespace).Delete(context.Background(), name, deleteOpts)
}

func (c *Job) GetByGUID(guid string, includeCompleted bool) ([]batchv1.Job, error) {
	labelSelector := fmt.Sprintf("%s=%s", c.getGUIDLabel(), guid)

	if !includeCompleted {
		labelSelector += fmt.Sprintf(",%s!=%s", k8s.LabelTaskCompleted, k8s.TaskCompletedTrue)
	}

	listOpts := metav1.ListOptions{LabelSelector: labelSelector}
	jobs, err := c.clientSet.BatchV1().Jobs(c.workloadsNamespace).List(context.Background(), listOpts)

	return jobs.Items, errors.Wrap(err, "failed to list jobs by guid")
}

func (c *Job) List(includeCompleted bool) ([]batchv1.Job, error) {
	labelSelector := fmt.Sprintf("%s=%s", k8s.LabelSourceType, c.jobType)

	if !includeCompleted {
		labelSelector += fmt.Sprintf(",%s!=%s", k8s.LabelTaskCompleted, k8s.TaskCompletedTrue)
	}

	listOpts := metav1.ListOptions{LabelSelector: labelSelector}
	jobs, err := c.clientSet.BatchV1().Jobs(c.workloadsNamespace).List(context.Background(), listOpts)

	return jobs.Items, errors.Wrap(err, "failed to list jobs")
}

func (c *Job) SetLabel(job *batchv1.Job, label, value string) (*batchv1.Job, error) {
	patchBytes := patching.NewLabel(label, value).GetJSONPatchBytes()

	return c.clientSet.BatchV1().Jobs(job.Namespace).Patch(
		context.Background(),
		job.Name,
		types.JSONPatchType,
		patchBytes, metav1.PatchOptions{})
}

func (c *Job) getGUIDLabel() string {
	if c.jobType == "TASK" {
		return k8s.LabelGUID
	}

	return k8s.LabelStagingGUID
}

type Secret struct {
	clientSet kubernetes.Interface
}

func NewSecret(clientSet kubernetes.Interface) *Secret {
	return &Secret{clientSet: clientSet}
}

func (c *Secret) Get(namespace, name string) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *Secret) Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

func (c *Secret) Update(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
}

func (c *Secret) Delete(namespace string, name string) error {
	return c.clientSet.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

type Event struct {
	clientSet kubernetes.Interface
}

func NewEvent(clientSet kubernetes.Interface) *Event {
	return &Event{
		clientSet: clientSet,
	}
}

func (c *Event) GetByPod(pod corev1.Pod) ([]corev1.Event, error) {
	eventList, err := c.clientSet.CoreV1().Events("").List(context.Background(), metav1.ListOptions{
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

func (c *Event) GetByInstanceAndReason(namespace string, ownerRef metav1.OwnerReference, instanceIndex int, reason string) (*corev1.Event, error) {
	fieldSelector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s,involvedObject.namespace=%s,reason=%s",
		ownerRef.Kind,
		ownerRef.Name,
		namespace,
		reason,
	)
	labelSelector := fmt.Sprintf("cloudfoundry.org/instance_index=%d", instanceIndex)

	kubeEvents, err := c.clientSet.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
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

func (c *Event) Create(namespace string, event *corev1.Event) (*corev1.Event, error) {
	return c.clientSet.CoreV1().Events(namespace).Create(context.Background(), event, metav1.CreateOptions{})
}

func (c *Event) Update(namespace string, event *corev1.Event) (*corev1.Event, error) {
	return c.clientSet.CoreV1().Events(namespace).Update(context.Background(), event, metav1.UpdateOptions{})
}
