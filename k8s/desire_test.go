package k8s_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/julz/cube/k8s"
	"github.com/julz/cube/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NOTE: this test requires a minikube to be set up and targeted in ~/.kube/config
var _ = Describe("Desiring some LRPs", func() {
	var (
		client    *kubernetes.Clientset
		desirer   *k8s.Desirer
		namespace string
		lrps      []opi.LRP
	)

	namespaceExists := func(name string) bool {
		_, err := client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
		return err == nil
	}

	createNamespace := func(name string) {
		namespaceSpec := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}

		if _, err := client.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
			panic(err)
		}
	}

	getLRPNames := func() []string {
		names := []string{}
		for _, lrp := range lrps {
			names = append(names, lrp.Name)
		}
		return names
	}

	BeforeEach(func() {
		config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err.Error())
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		namespace = "testing"
		lrps = []opi.LRP{
			{Name: "app0", Image: "busybox", TargetInstances: 1, Command: []string{""}},
			{Name: "app1", Image: "busybox", TargetInstances: 3, Command: []string{""}},
		}
	})

	JustBeforeEach(func() {
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		desirer = &k8s.Desirer{
			KubeNamespace: namespace,
			Client:        client,
		}
	})

	AfterEach(func() {
		for _, appName := range getLRPNames() {
			if err := client.AppsV1beta1().Deployments(namespace).Delete(appName, &metav1.DeleteOptions{}); err != nil {
				panic(err)
			}

			if err := client.CoreV1().Services(namespace).Delete(appName, &metav1.DeleteOptions{}); err != nil {
				panic(err)
			}
		}
	})

	Context("When a LPP is desired", func() {

		getDeploymentNames := func(deployments *v1beta1.DeploymentList) []string {
			depNames := []string{}
			for _, deployment := range deployments.Items {
				depNames = append(depNames, deployment.ObjectMeta.Name)
			}

			return depNames
		}

		It("Creates deployments for every LRP in the array", func() {
			Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

			deployments, err := client.AppsV1beta1().Deployments(namespace).List(av1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(deployments.Items).To(HaveLen(len(lrps)))
			Expect(getDeploymentNames(deployments)).To(ConsistOf(getLRPNames()))
		})

		It("creates services for every deployment", func() {
			Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

			services, err := client.CoreV1().Services(namespace).List(av1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(services.Items).To(HaveLen(len(lrps)))
		})

		It("Doesn't error when the deployment already exists", func() {
			for i := 0; i < 2; i++ {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())
			}
		})
	})

	PIt("Removes any LRPs in the namespace that are no longer desired", func() {
	})

	PIt("Updates any LRPs whose etag annotation has changed", func() {
	})
})
