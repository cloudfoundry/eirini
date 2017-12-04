package k8s_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/julz/cube/k8s"
	"github.com/julz/cube/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NOTE: this test requires a minikube to be set up and targeted in ~/.kube/config
var _ = Describe("Desiring some LRPs", func() {
	var (
		client  *kubernetes.Clientset
		desirer *k8s.Desirer
	)

	BeforeEach(func() {
		config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err.Error())
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		desirer = &k8s.Desirer{Client: client}
	})

	It("Creates deployments for every LRP in the array", func() {
		Expect(desirer.Desire(context.Background(), []opi.LRP{
			{Name: "app0", Image: "busybox", TargetInstances: 1},
			{Name: "app1", Image: "busybox", TargetInstances: 3},
		})).To(Succeed())

		deployments, err := client.AppsV1beta1().Deployments("default").List(av1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(deployments.Items).To(HaveLen(2))
	})

	It("Doesn't error when the deployment already exists", func() {
		for i := 0; i < 2; i++ {
			Expect(desirer.Desire(context.Background(), []opi.LRP{
				{Name: "app0", Image: "busybox", TargetInstances: 1},
				{Name: "app1", Image: "busybox", TargetInstances: 3},
			})).To(Succeed())
		}
	})

	PIt("Removes any LRPs in the namespace that are no longer desired", func() {
	})

	PIt("Updates any LRPs whose etag annotation has changed", func() {
	})
})
