package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyv1beta1_types "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	fixture *tests.Fixture
	ctx     context.Context
)

var _ = BeforeSuite(func() {
	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = BeforeEach(func() {
	fixture.SetUp()
	ctx = context.Background()
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

func labelSelector(identifier opi.LRPIdentifier) string {
	return fmt.Sprintf(
		"%s=%s,%s=%s",
		stset.LabelGUID, identifier.GUID,
		stset.LabelVersion, identifier.Version,
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
			stset.LabelGUID, lrp1.LRPIdentifier.GUID, lrp2.LRPIdentifier.GUID,
			stset.LabelVersion, lrp1.LRPIdentifier.Version, lrp2.LRPIdentifier.Version,
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

func listPodsByLabel(labelSelector string) []corev1.Pod {
	pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())

	return pods.Items
}

func listPods(lrpIdentifier opi.LRPIdentifier) []corev1.Pod {
	return listPodsByLabel(labelSelector(lrpIdentifier))
}

func podNamesFromPods(pods []corev1.Pod) []string {
	names := []string{}
	for _, p := range pods {
		names = append(names, p.Name)
	}

	return names
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
		Image:           "eirini/busybox",
		AppURIs:         []opi.Route{{Hostname: "foo.example.com", Port: 8080}},
		LRPIdentifier:   opi.LRPIdentifier{GUID: tests.GenerateGUID(), Version: tests.GenerateGUID()},
		LRP:             "metadata",
		DiskMB:          2047,
	}
}

func getSecret(ns, name string) (*corev1.Secret, error) {
	return fixture.Clientset.CoreV1().Secrets(ns).Get(context.Background(), name, metav1.GetOptions{})
}
