package k8s_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta1"
	v1beta2 "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "code.cloudfoundry.org/eirini/k8s"
)

// NOTE: this test requires a minikube to be set up and targeted in ~/.kube/config
var _ = Describe("Deployment", func() {

	var (
		client            kubernetes.Interface
		deploymentManager DeploymentManager
		lrps              []opi.LRP
	)

	const (
		namespace               = "testing"
		timeout   time.Duration = 60 * time.Second
	)

	namespaceExists := func(name string) bool {
		_, err := client.CoreV1().Namespaces().Get(namespace, av1.GetOptions{})
		return err == nil
	}

	createNamespace := func(name string) {
		namespaceSpec := &v1.Namespace{
			ObjectMeta: av1.ObjectMeta{Name: name},
		}

		if _, err := client.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
			panic(err)
		}
	}

	getLRPNames := func() []string {
		names := []string{}
		for _, lrp := range lrps {
			names = append(names, lrp.Metadata[cf.ProcessGUID])
		}
		return names
	}

	listDeployments := func() []v1beta1.Deployment {
		list, err := client.AppsV1beta1().Deployments(namespace).List(av1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		return list.Items
	}

	listServices := func() []v1.Service {
		list, err := client.CoreV1().Services(namespace).List(av1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		return list.Items
	}

	listPods := func(appName string) []v1.Pod {
		labelSelector := fmt.Sprintf("name=%s", appName)
		pods, err := client.CoreV1().Pods(namespace).List(av1.ListOptions{LabelSelector: labelSelector})
		Expect(err).NotTo(HaveOccurred())
		return pods.Items
	}

	listReplicasets := func(appName string) []v1beta2.ReplicaSet {
		labelSelector := fmt.Sprintf("name=%s", appName)
		replicasets, err := client.AppsV1beta2().ReplicaSets(namespace).List(av1.ListOptions{LabelSelector: labelSelector})
		Expect(err).NotTo(HaveOccurred())
		return replicasets.Items
	}

	cleanupDeployment := func(appName string) {
		backgroundPropagation := av1.DeletePropagationBackground
		client.AppsV1beta1().Deployments(namespace).Delete(appName, &av1.DeleteOptions{PropagationPolicy: &backgroundPropagation})
	}

	cleanupService := func(appName string) {
		serviceName := eirini.GetInternalServiceName(appName)
		client.CoreV1().Services(namespace).Delete(serviceName, &av1.DeleteOptions{})
	}

	createClient := func() {
		config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err.Error())
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
	}

	BeforeEach(func() {
		lrps = []opi.LRP{
			createLRP("odin", "1234.5"),
			createLRP("thor", "4567.8"),
			createLRP("mimir", "9012.3"),
		}
	})

	AfterEach(func() {
		for _, appName := range getLRPNames() {
			cleanupDeployment(appName)
			cleanupService(appName)
		}

		Eventually(listDeployments, timeout).Should(BeEmpty())
		Eventually(listServices, timeout).Should(BeEmpty())
	})

	JustBeforeEach(func() {
		createClient()
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		for _, l := range lrps {
			client.AppsV1beta1().Deployments(namespace).Create(toDeployment(l))
		}

		deploymentManager = NewDeploymentManager(client)
	})

	Context("List deployments", func() {

		It("translates all existing deployments to opi.LRPs", func() {
			actualLRPs, err := deploymentManager.ListLRPs(namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualLRPs).To(ConsistOf(lrps))
		})

		Context("When no deployments exist", func() {

			BeforeEach(func() {
				lrps = []opi.LRP{}
			})

			It("returns an empy list of LRPs", func() {
				actualLRPs, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualLRPs).To(BeEmpty())
			})
		})
	})

	Context("Delete a deployment", func() {

		It("deletes the deployment", func() {
			err := deploymentManager.Delete("odin", namespace)
			Expect(err).ToNot(HaveOccurred())

			Eventually(listDeployments, timeout).Should(HaveLen(2))
			Expect(getDeploymentNames(listDeployments())).To(ConsistOf("mimir", "thor"))
		})

		It("deletes the pods associated with the deployment", func() {
			err := deploymentManager.Delete("odin", namespace)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []v1.Pod {
				return listPods("odin")
			}, timeout).Should(BeEmpty())
		})

		It("deletes the replicasets associated with the deployment", func() {
			err := deploymentManager.Delete("odin", namespace)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []v1beta2.ReplicaSet {
				return listReplicasets("odin")
			}, timeout).Should(BeEmpty())
		})

		Context("when the deployment does not exist", func() {

			It("returns an error", func() {
				err := deploymentManager.Delete("test-app-where-are-you", namespace)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func getDeploymentNames(deployments []v1beta1.Deployment) []string {
	deploymentNames := []string{}
	for _, d := range deployments {
		deploymentNames = append(deploymentNames, d.Name)
	}
	return deploymentNames
}

func toDeployment(lrp opi.LRP) *v1beta1.Deployment {
	replicas := int32(2)
	deployment := &v1beta1.Deployment{
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:    "cont",
							Image:   "busybox",
							Command: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 10;done"},
							Env:     []v1.EnvVar{v1.EnvVar{Name: "GOPATH", Value: "~/go"}},
						},
					},
				},
			},
		},
	}

	deployment.Name = lrp.Metadata[cf.ProcessGUID]
	deployment.Spec.Template.Labels = map[string]string{
		"name": lrp.Metadata[cf.ProcessGUID],
	}
	deployment.Annotations = map[string]string{
		cf.ProcessGUID: lrp.Metadata[cf.ProcessGUID],
		cf.LastUpdated: lrp.Metadata[cf.LastUpdated],
	}

	return deployment
}

func createLRP(processGUID, lastUpdated string) opi.LRP {
	return opi.LRP{
		Metadata: map[string]string{
			cf.ProcessGUID: processGUID,
			cf.LastUpdated: lastUpdated,
		},
	}
}
