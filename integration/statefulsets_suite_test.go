package statefulsets_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1_types "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1_types "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	fixture *util.Fixture
)

const (
	timeout time.Duration = 60 * time.Second
)

var _ = BeforeSuite(func() {
	fixture = util.NewFixture(GinkgoWriter)
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func secrets() corev1_types.SecretInterface {
	return fixture.Clientset.CoreV1().Secrets(fixture.Namespace)
}

func getSecret(name string) (*corev1.Secret, error) {
	return secrets().Get(name, metav1.GetOptions{})
}

func statefulSets() appsv1_types.StatefulSetInterface {
	return fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace)
}

func getStatefulSet(lrp *opi.LRP) *appsv1.StatefulSet {
	ss, getErr := statefulSets().List(metav1.ListOptions{LabelSelector: labelSelector(lrp.LRPIdentifier)})
	Expect(getErr).NotTo(HaveOccurred())
	return &ss.Items[0]
}

func labelSelector(identifier opi.LRPIdentifier) string {
	return fmt.Sprintf(
		"%s=%s,%s=%s",
		k8s.LabelGUID, identifier.GUID,
		k8s.LabelVersion, identifier.Version,
	)
}

func podDisruptionBudgets() v1beta1.PodDisruptionBudgetInterface {
	return fixture.Clientset.PolicyV1beta1().PodDisruptionBudgets(fixture.Namespace)
}

func cleanupStatefulSet(lrp *opi.LRP) {
	backgroundPropagation := metav1.DeletePropagationBackground
	deleteOptions := &metav1.DeleteOptions{PropagationPolicy: &backgroundPropagation}
	listOptions := metav1.ListOptions{LabelSelector: labelSelector(lrp.LRPIdentifier)}
	err := statefulSets().DeleteCollection(deleteOptions, listOptions)
	Expect(err).ToNot(HaveOccurred())
}

func listAllStatefulSets(lrp1, lrp2 *opi.LRP) []appsv1.StatefulSet {
	labels := fmt.Sprintf(
		"%s in (%s, %s),%s in (%s, %s)",
		k8s.LabelGUID, lrp1.LRPIdentifier.GUID, lrp2.LRPIdentifier.GUID,
		k8s.LabelVersion, lrp1.LRPIdentifier.Version, lrp2.LRPIdentifier.Version,
	)

	list, err := statefulSets().List(metav1.ListOptions{LabelSelector: labels})
	Expect(err).NotTo(HaveOccurred())
	return list.Items
}

func listStatefulSets(appName string) []appsv1.StatefulSet {
	labelSelector := fmt.Sprintf("name=%s", appName)
	list, err := statefulSets().List(metav1.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())
	return list.Items
}

func listPodsByLabel(labelSelector string) []corev1.Pod {
	pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
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
	nodeList, err := fixture.Clientset.CoreV1().Nodes().List(metav1.ListOptions{})
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
	guid := util.RandomString()
	routes, err := json.Marshal([]cf.Route{{Hostname: "foo.example.com", Port: 8080}})
	Expect(err).ToNot(HaveOccurred())
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
		AppURIs:         string(routes),
		LRPIdentifier:   opi.LRPIdentifier{GUID: guid, Version: "version_" + guid},
		LRP:             "metadata",
		DiskMB:          2047,
	}
}
