package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	namespace string
	clientset kubernetes.Interface
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
