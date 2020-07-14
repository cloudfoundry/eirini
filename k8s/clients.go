package k8s

import (
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
	return c.clientSet.CoreV1().Pods("").List(opts)
}

func (c *podsClient) Delete(namespace, name string) error {
	return c.clientSet.CoreV1().Pods(namespace).Delete(name, nil)
}

type podDisruptionBudgetClient struct {
	clientSet kubernetes.Interface
}

func NewPodDisruptionBudgetClient(clientSet kubernetes.Interface) PodDisruptionBudgetClient {
	return &podDisruptionBudgetClient{clientSet: clientSet}
}

func (c *podDisruptionBudgetClient) Create(namespace string, podDisruptionBudget *v1beta1.PodDisruptionBudget) (*v1beta1.PodDisruptionBudget, error) {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Create(podDisruptionBudget)
}

func (c *podDisruptionBudgetClient) Delete(namespace string, name string) error {
	return c.clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Delete(name, nil)
}

type statefulSetClient struct {
	clientSet kubernetes.Interface
}

func NewStatefulSetClient(clientSet kubernetes.Interface) StatefulSetClient {
	return &statefulSetClient{clientSet: clientSet}
}

func (c *statefulSetClient) Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Create(statefulSet)
}

func (c *statefulSetClient) Update(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return c.clientSet.AppsV1().StatefulSets(namespace).Update(statefulSet)
}

func (c *statefulSetClient) Delete(namespace string, name string, options *metav1.DeleteOptions) error {
	return c.clientSet.AppsV1().StatefulSets(namespace).Delete(name, options)
}

func (c *statefulSetClient) List(opts metav1.ListOptions) (*appsv1.StatefulSetList, error) {
	return c.clientSet.AppsV1().StatefulSets("").List(opts)
}

type JobClient struct {
	clientSet kubernetes.Interface
}

func NewJobClient(clientSet kubernetes.Interface) *JobClient {
	return &JobClient{clientSet: clientSet}
}

func (c *JobClient) Create(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Create(job)
}

func (c *JobClient) Update(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return c.clientSet.BatchV1().Jobs(namespace).Update(job)
}

func (c *JobClient) Delete(namespace string, name string, options *metav1.DeleteOptions) error {
	return c.clientSet.BatchV1().Jobs(namespace).Delete(name, options)
}

func (c *JobClient) List(opts metav1.ListOptions) (*batchv1.JobList, error) {
	return c.clientSet.BatchV1().Jobs("").List(opts)
}

type SecretsClient struct {
	clientSet kubernetes.Interface
}

func NewSecretsClient(clientSet kubernetes.Interface) *SecretsClient {
	return &SecretsClient{clientSet: clientSet}
}

func (c *SecretsClient) Get(namespace, name string) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
}

func (c *SecretsClient) Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Create(secret)
}

func (c *SecretsClient) Update(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientSet.CoreV1().Secrets(namespace).Update(secret)
}

func (c *SecretsClient) Delete(namespace string, name string) error {
	return c.clientSet.CoreV1().Secrets(namespace).Delete(name, nil)
}

type eventsClient struct {
	clientSet kubernetes.Interface
}

func NewEventsClient(clientSet kubernetes.Interface) EventLister {
	return &eventsClient{clientSet: clientSet}
}

func (c *eventsClient) List(opts metav1.ListOptions) (*corev1.EventList, error) {
	return c.clientSet.CoreV1().Events("").List(opts)
}
