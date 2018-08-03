package integration_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	timeout time.Duration = 60 * time.Second
)

var (
	namespace          string
	clientset          kubernetes.Interface
	validOpiConfigPath string
)

// nolint
var _ = BeforeSuite(func() {
	namespace = "opi-integration"
	config, err := clientcmd.BuildConfigFromFlags("",
		filepath.Join(os.Getenv("HOME"), ".kube", "config"),
	)
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	clientset.CoreV1().Namespaces().Create(ns)
})

func init() {
	flag.StringVar(&validOpiConfigPath, "valid_opi_config", "", "path to a valid opi config file")
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}
