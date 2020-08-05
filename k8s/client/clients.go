package client

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Pod struct {
	clientSet kubernetes.Interface
}

func NewPod(clientSet kubernetes.Interface) *Pod {
	return &Pod{clientSet: clientSet}
}

func (c *Pod) List(opts metav1.ListOptions) (*corev1.PodList, error) {
	return c.clientSet.CoreV1().Pods("").List(context.Background(), opts)
}

func (c *Pod) Delete(namespace, name string) error {
	return c.clientSet.CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

type PodDisruptionBudget struct {
	clientSet kubernetes.Interface
}

func NewPodDisruptionBudget(clientSet kubernetes.Interface) *PodDisruptionBudget {
	return &PodDisruptionBudget{clientSet: clientSet}
}

func (c *PodDisruptionBudget) Create(namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error) {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Create(context.Background(), podDisruptionBudget, metav1.CreateOptions{})
}

func (c *PodDisruptionBudget) Delete(namespace string, name string) error {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

type StatefulSet struct {
	clientSet kubernetes.Interface
}

func NewStatefulSet(clientSet kubernetes.Interface) *StatefulSet {
	return &StatefulSet{clientSet: clientSet}
}

func (c *StatefulSet) Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Create(context.Background(), statefulSet, metav1.CreateOptions{})
}

func (c *StatefulSet) Get(namespace, name string) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *StatefulSet) Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Update(context.Background(), statefulSet, metav1.UpdateOptions{})
}

func (c *StatefulSet) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return c.clientSet.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, options)
}

func (c *StatefulSet) List(opts metav1.ListOptions) (*appsv1.StatefulSetList, error) {
	return c.clientSet.AppsV1().StatefulSets("").List(context.Background(), opts)
}

type Job struct {
	clientSet kubernetes.Interface
}

func NewJob(clientSet kubernetes.Interface) *Job {
	return &Job{clientSet: clientSet}
}

func (c *Job) Create(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
}

func (c *Job) Update(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Update(context.Background(), job, metav1.UpdateOptions{})
}

func (c *Job) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return c.clientSet.BatchV1().Jobs(namespace).Delete(context.Background(), name, options)
}

func (c *Job) List(opts metav1.ListOptions) (*batchv1.JobList, error) {
	return c.clientSet.BatchV1().Jobs("").List(context.Background(), opts)
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
	return &Event{clientSet: clientSet}
}

func (c *Event) List(opts metav1.ListOptions) (*corev1.EventList, error) {
	return c.clientSet.CoreV1().Events("").List(context.Background(), opts)
}

func (c *Event) Create(namespace string, event *corev1.Event) (*corev1.Event, error) {
	ctx := context.Background()
	return c.clientSet.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
}

func (c *Event) Update(namespace string, event *corev1.Event) (*corev1.Event, error) {
	return c.clientSet.CoreV1().Events(namespace).Update(context.Background(), event, metav1.UpdateOptions{})
}
