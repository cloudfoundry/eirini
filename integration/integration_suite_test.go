package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyv1beta1_types "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var fixture *util.Fixture

var _ = BeforeSuite(func() {
	fixture = util.NewFixture(GinkgoWriter)
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})

var _ = AfterSuite(func() {
	fixture.Destroy()
})

func TestIntegration(t *testing.T) {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func getStatefulSetForLRP(lrp *opi.LRP) *appsv1.StatefulSet {
	ss, getErr := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector(lrp.LRPIdentifier),
	})
	Expect(getErr).NotTo(HaveOccurred())

	return &ss.Items[0]
}

func getStatefulSet(ns, name string) *appsv1.StatefulSet {
	ss, err := fixture.Clientset.AppsV1().StatefulSets(ns).Get(context.Background(), name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return ss
}

func labelSelector(identifier opi.LRPIdentifier) string {
	return fmt.Sprintf(
		"%s=%s,%s=%s",
		k8s.LabelGUID, identifier.GUID,
		k8s.LabelVersion, identifier.Version,
	)
}

func podDisruptionBudgets() policyv1beta1_types.PodDisruptionBudgetInterface {
	return fixture.Clientset.PolicyV1beta1().PodDisruptionBudgets(fixture.Namespace)
}

func cleanupStatefulSet(lrp *opi.LRP) {
	backgroundPropagation := metav1.DeletePropagationBackground
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: &backgroundPropagation}
	listOptions := metav1.ListOptions{LabelSelector: labelSelector(lrp.LRPIdentifier)}
	err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).DeleteCollection(context.Background(), deleteOptions, listOptions)
	Expect(err).ToNot(HaveOccurred())
}

func listAllStatefulSets(lrp1, lrp2 *opi.LRP) []appsv1.StatefulSet {
	list, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s in (%s, %s),%s in (%s, %s)",
			k8s.LabelGUID, lrp1.LRPIdentifier.GUID, lrp2.LRPIdentifier.GUID,
			k8s.LabelVersion, lrp1.LRPIdentifier.Version, lrp2.LRPIdentifier.Version,
		),
	})
	Expect(err).NotTo(HaveOccurred())

	return list.Items
}

func listStatefulSetsForApp(appName string) []appsv1.StatefulSet {
	list, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s", appName),
	})
	Expect(err).NotTo(HaveOccurred())

	return list.Items
}

func listStatefulSets(ns string) []appsv1.StatefulSet {
	statfulSets, err := fixture.Clientset.AppsV1().StatefulSets(ns).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	return statfulSets.Items
}

func listPodsByLabel(labelSelector string) []corev1.Pod {
	pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())

	return pods.Items
}

func listPods(lrpIdentifier opi.LRPIdentifier) []corev1.Pod {
	return listPodsByLabel(labelSelector(lrpIdentifier))
}

func listAllPods() []corev1.Pod {
	return listPodsByLabel("")
}

func listJobs(ns string) []batchv1.Job {
	jobs, err := fixture.Clientset.BatchV1().Jobs(ns).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	return jobs.Items
}

func podNamesFromPods(pods []corev1.Pod) []string {
	names := []string{}
	for _, p := range pods {
		names = append(names, p.Name)
	}

	return names
}

func createPods(ns string, names ...string) {
	for _, name := range names {
		createPod(ns, name, map[string]string{})
	}
}

func createPod(ns, name string, labels map[string]string) {
	_, err := fixture.Clientset.CoreV1().Pods(ns).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "busybox",
					Image: "busybox",
				},
			},
		},
	}, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred())
}

func nodeNamesFromPods(pods []corev1.Pod) []string {
	names := []string{}

	for _, p := range pods {
		nodeName := p.Spec.NodeName
		if nodeName != "" {
			names = append(names, nodeName)
		}
	}

	return names
}

func getNodeCount() int {
	nodeList, err := fixture.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())

	return len(nodeList.Items)
}

func podCrashed(pod corev1.Pod) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false
	}

	terminated := pod.Status.ContainerStatuses[0].State.Terminated
	waiting := pod.Status.ContainerStatuses[0].State.Waiting

	return terminated != nil || waiting != nil && waiting.Reason == "CrashLoopBackOff"
}

func podReady(pod corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

func createLRP(name string) *opi.LRP {
	return &opi.LRP{
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		AppName:         name,
		SpaceName:       "space-foo",
		TargetInstances: 2,
		Image:           "busybox",
		AppURIs:         []opi.Route{{Hostname: "foo.example.com", Port: 8080}},
		LRPIdentifier:   opi.LRPIdentifier{GUID: util.GenerateGUID(), Version: util.GenerateGUID()},
		LRP:             "metadata",
		DiskMB:          2047,
	}
}

func listPDBs(ns string) []policyv1beta1.PodDisruptionBudget {
	pdbs, err := fixture.Clientset.PolicyV1beta1().PodDisruptionBudgets(ns).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	return pdbs.Items
}

func createPDB(ns, name string) {
	_, err := fixture.Clientset.PolicyV1beta1().PodDisruptionBudgets(ns).Create(
		context.Background(),
		&policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
		metav1.CreateOptions{},
	)
	Expect(err).NotTo(HaveOccurred())
}

func createStatefulSet(ns, name string, labels map[string]string) *appsv1.StatefulSet {
	id := util.GenerateGUID()
	statefulSet, err := fixture.Clientset.AppsV1().StatefulSets(ns).Create(context.Background(), &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"id": id,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"id": id,
					},
				},
			},
		},
	}, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred())

	return statefulSet
}

func createSecret(ns, name string, labels map[string]string) *corev1.Secret {
	secret, err := fixture.Clientset.CoreV1().Secrets(ns).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	return secret
}

func createJob(ns, name string, labels map[string]string) *batchv1.Job {
	runAsNonRoot := true
	runAsUser := int64(2000)
	statefulSet, err := fixture.Clientset.BatchV1().Jobs(ns).Create(context.Background(), &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
					},
					Containers: []corev1.Container{
						{
							Name:            "test",
							Image:           "busybox",
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"echo", "hi"},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred())

	return statefulSet
}

func listSecrets(ns string) []corev1.Secret {
	secrets, err := fixture.Clientset.CoreV1().Secrets(ns).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	return secrets.Items
}

func getSecret(ns, name string) (*corev1.Secret, error) {
	return fixture.Clientset.CoreV1().Secrets(ns).Get(context.Background(), name, metav1.GetOptions{})
}
