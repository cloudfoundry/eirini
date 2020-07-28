package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type podsClient struct {
	clientSet kubernetes.Interface
}

func NewPodsClient(clientSet kubernetes.Interface) PodListerDeleter {
	return &podsClient{clientSet: clientSet}
}

func (c *podsClient) List(opts metav1.ListOptions) (*corev1.PodList, error) {
	return c.clientSet.CoreV1().Pods("").List(context.Background(), opts)
}

func (c *podsClient) Delete(namespace, name string) error {
	return c.clientSet.CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

type podDisruptionBudgetClient struct {
	clientSet kubernetes.Interface
}

func NewPodDisruptionBudgetClient(clientSet kubernetes.Interface) PodDisruptionBudgetClient {
	return &podDisruptionBudgetClient{clientSet: clientSet}
}

func (c *podDisruptionBudgetClient) Create(namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error) {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Create(context.Background(), podDisruptionBudget, metav1.CreateOptions{})
}

func (c *podDisruptionBudgetClient) Delete(namespace string, name string) error {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

type StatefulSetClient struct {
	clientSet kubernetes.Interface
}

func NewStatefulSetClient(clientSet kubernetes.Interface) *StatefulSetClient {
	return &StatefulSetClient{clientSet: clientSet}
}

func (c *StatefulSetClient) Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Create(context.Background(), statefulSet, metav1.CreateOptions{})
}

func (c *StatefulSetClient) Get(namespace, name string) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *StatefulSetClient) Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Update(context.Background(), statefulSet, metav1.UpdateOptions{})
}

func (c *StatefulSetClient) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return c.clientSet.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, options)
}

func (c *StatefulSetClient) List(opts metav1.ListOptions) (*appsv1.StatefulSetList, error) {
	return c.clientSet.AppsV1().StatefulSets("").List(context.Background(), opts)
}

type JobClient struct {
	clientSet kubernetes.Interface
}

func NewJobClient(clientSet kubernetes.Interface) *JobClient {
	return &JobClient{clientSet: clientSet}
}

func (c *JobClient) Create(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
}

func (c *JobClient) Update(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Update(context.Background(), job, metav1.UpdateOptions{})
}

func (c *JobClient) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return c.clientSet.BatchV1().Jobs(namespace).Delete(context.Background(), name, options)
}

func (c *JobClient) List(opts metav1.ListOptions) (*batchv1.JobList, error) {
	return c.clientSet.BatchV1().Jobs("").List(context.Background(), opts)
}

type SecretsClient struct {
	clientSet kubernetes.Interface
}

func NewSecretsClient(clientSet kubernetes.Interface) *SecretsClient {
	return &SecretsClient{clientSet: clientSet}
}

func (c *SecretsClient) Get(namespace, name string) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (c *SecretsClient) Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

func (c *SecretsClient) Update(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
}

func (c *SecretsClient) Delete(namespace string, name string) error {
	return c.clientSet.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

type eventsClient struct {
	clientSet kubernetes.Interface
}

func NewEventsClient(clientSet kubernetes.Interface) EventLister {
	return &eventsClient{clientSet: clientSet}
}

func (c *eventsClient) List(ctx context.Context, opts metav1.ListOptions) (*corev1.EventList, error) {
	return c.clientSet.CoreV1().Events("").List(ctx, opts)
}
