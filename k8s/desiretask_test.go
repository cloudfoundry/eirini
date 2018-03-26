package k8s_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/julz/cube/k8s"
	"github.com/julz/cube/opi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desiretask", func() {
	var (
		client  *kubernetes.Clientset
		desirer *k8s.TaskDesirer
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

		desirer = &k8s.TaskDesirer{Client: client}
	})

	AfterEach(func() {
		desirer.DeleteJob("guid")
	})

	It("creates jobs for every task in the array", func() {
		Expect(desirer.Desire(context.Background(), []opi.Task{
			{Image: "pi", Command: []string{}, Env: map[string]string{"APP_ID": "test", "STAGING_GUID": "guid"}},
		})).To(Succeed())

		deployments, err := client.AppsV1beta1().Deployments("default").List(av1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(deployments.Items).To(HaveLen(2))
	})
})
