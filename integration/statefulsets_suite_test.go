package statefulsets_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	coretypes "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	namespace string
	clientset kubernetes.Interface
)

const (
	timeout time.Duration = 60 * time.Second
)

var _ = BeforeSuite(func() {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("INTEGRATION_KUBECONFIG"))
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	namespace = "opi-integration-test"

	if !namespaceExists() {
		createNamespace()
	}
})

var _ = AfterSuite(func() {
	err := clientset.CoreV1().Namespaces().Delete(namespace, &meta.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func namespaceExists() bool {
	_, err := clientset.CoreV1().Namespaces().Get(namespace, meta.GetOptions{})
	return err == nil
}

func createNamespace() {
	namespaceSpec := &v1.Namespace{ObjectMeta: meta.ObjectMeta{Name: namespace}}

	if _, err := clientset.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
		panic(err)
	}
}

func statefulSets() types.StatefulSetInterface {
	return clientset.AppsV1beta2().StatefulSets(namespace)
}

func services() coretypes.ServiceInterface {
	return clientset.CoreV1().Services(namespace)
}

func getStatefulSet(lrp *opi.LRP) *v1beta2.StatefulSet {
	ss, getErr := clientset.AppsV1beta2().StatefulSets(namespace).List(meta.ListOptions{LabelSelector: labelSelector(lrp)})
	Expect(getErr).NotTo(HaveOccurred())
	return &ss.Items[0]
}

func labelSelector(lrp *opi.LRP) string {
	return fmt.Sprintf("guid=%s,version=%s", lrp.LRPIdentifier.GUID, lrp.LRPIdentifier.Version)
}

func cleanupStatefulSet(lrp *opi.LRP) {
	backgroundPropagation := meta.DeletePropagationBackground
	deleteOptions := &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation}
	listOptions := meta.ListOptions{LabelSelector: labelSelector(lrp)}
	err := statefulSets().DeleteCollection(deleteOptions, listOptions)
	Expect(err).ToNot(HaveOccurred())
}

func listAllStatefulSets() []v1beta2.StatefulSet {
	list, err := clientset.AppsV1beta2().StatefulSets(namespace).List(meta.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	return list.Items
}

func listStatefulSets(appName string) []v1beta2.StatefulSet {
	labelSelector := fmt.Sprintf("name=%s", appName)
	list, err := clientset.AppsV1beta2().StatefulSets(namespace).List(meta.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())
	return list.Items
}

func listPodsByLabel(labelSelector string) []v1.Pod {
	pods, err := clientset.CoreV1().Pods(namespace).List(meta.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())
	return pods.Items
}

func listPods(lrpIdentifier opi.LRPIdentifier) []v1.Pod {
	labelSelector := fmt.Sprintf("guid=%s,version=%s", lrpIdentifier.GUID, lrpIdentifier.Version)
	return listPodsByLabel(labelSelector)
}

func podNamesFromPods(pods []v1.Pod) []string {
	names := []string{}
	for _, p := range pods {
		names = append(names, p.Name)
	}
	return names
}
