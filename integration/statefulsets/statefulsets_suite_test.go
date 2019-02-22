package statefulsets_test

import (
	"fmt"
	"os"
	"path/filepath"
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
	config, err := clientcmd.BuildConfigFromFlags("",
		filepath.Join(os.Getenv("HOME"), ".kube", "config"),
	)
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	namespace = "opi-integration"
	if !namespaceExists() {
		createNamespace()
	}
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
	namespaceSpec := &v1.Namespace{
		ObjectMeta: meta.ObjectMeta{Name: namespace},
	}

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

func cleanupStatefulSet(appName string) {
	if _, err := statefulSets().Get(appName, meta.GetOptions{}); err == nil {
		backgroundPropagation := meta.DeletePropagationBackground
		err = statefulSets().Delete(appName, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation})
		Expect(err).ToNot(HaveOccurred())
	}
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

func listPods(lrpIdentifier opi.LRPIdentifier) []v1.Pod {
	labelSelector := fmt.Sprintf("guid=%s,version=%s", lrpIdentifier.GUID, lrpIdentifier.Version)
	pods, err := clientset.CoreV1().Pods(namespace).List(meta.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())
	return pods.Items
}

func getPodNames(lrpIdentifier opi.LRPIdentifier) []string {
	names := []string{}
	for _, p := range listPods(lrpIdentifier) {
		names = append(names, p.Name)
	}
	return names
}
