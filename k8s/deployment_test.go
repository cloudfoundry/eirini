package k8s_test

import (
	"fmt"
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
	"k8s.io/client-go/kubernetes/fake"

	. "code.cloudfoundry.org/eirini/k8s"
)

var _ = Describe("Deployment", func() {

	var (
		err               error
		client            kubernetes.Interface
		deploymentManager InstanceManager
		lrps              []*opi.LRP
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

	//nolint
	cleanupDeployment := func(appName string) {
		backgroundPropagation := av1.DeletePropagationBackground
		client.AppsV1beta1().Deployments(namespace).Delete(appName, &av1.DeleteOptions{PropagationPolicy: &backgroundPropagation})
	}

	//nolint
	cleanupService := func(appName string) {
		serviceName := eirini.GetInternalServiceName(appName)
		client.CoreV1().Services(namespace).Delete(serviceName, &av1.DeleteOptions{})
	}

	BeforeEach(func() {
		lrps = []*opi.LRP{
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

	//nolint
	JustBeforeEach(func() {
		client = fake.NewSimpleClientset()
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		for _, l := range lrps {
			client.AppsV1beta1().Deployments(namespace).Create(toDeployment(l, namespace))
		}

		deploymentManager = NewDeploymentManager(namespace, client)
	})

	Context("When creating an LRP", func() {
		var lrp *opi.LRP

		JustBeforeEach(func() {
			lrp = createLRP("Baldur", "1234.5")
			lrps = append(lrps, lrp)

			err = deploymentManager.Create(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the desired deployment", func() {
			deployment, err := client.AppsV1beta1().Deployments(namespace).Get("Baldur", av1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(deployment).To(Equal(toDeployment(lrp, namespace)))
		})

		Context("When redeploying an existing LRP", func() {
			BeforeEach(func() {
				lrp = createLRP("Baldur", "1234.5")
				lrps = append(lrps, lrp)
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("List/Delete", func() {
		Context("List deployments", func() {

			It("translates all existing deployments to opi.LRPs", func() {
				actualLRPs, err := deploymentManager.List()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualLRPs).To(ConsistOf(lrps))
			})

			Context("When no deployments exist", func() {

				BeforeEach(func() {
					lrps = []*opi.LRP{}
				})

				It("returns an empy list of LRPs", func() {
					actualLRPs, err := deploymentManager.List()
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPs).To(BeEmpty())
				})
			})
		})

		Context("Delete a deployment", func() {

			It("deletes the deployment", func() {
				err := deploymentManager.Delete("odin")
				Expect(err).ToNot(HaveOccurred())

				Eventually(listDeployments, timeout).Should(HaveLen(2))
				Expect(getDeploymentNames(listDeployments())).To(ConsistOf("mimir", "thor"))
			})

			PIt("deletes the pods associated with the deployment", func() {
				err := deploymentManager.Delete("odin")
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() []v1.Pod {
					return listPods("odin")
				}, timeout).Should(BeEmpty())
			})

			It("deletes the replicasets associated with the deployment", func() {
				err := deploymentManager.Delete("odin")
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() []v1beta2.ReplicaSet {
					return listReplicasets("odin")
				}, timeout).Should(BeEmpty())
			})

			Context("when the deployment does not exist", func() {

				It("returns an error", func() {
					err := deploymentManager.Delete("test-app-where-are-you")
					Expect(err).To(HaveOccurred())
				})
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

func toDeployment(lrp *opi.LRP, namespace string) *v1beta1.Deployment {
	targetInstances := int32(lrp.TargetInstances)
	deployment := &v1beta1.Deployment{
		Spec: v1beta1.DeploymentSpec{
			Replicas: &targetInstances,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     MapToEnvVar(lrp.Env),
							Ports: []v1.ContainerPort{
								v1.ContainerPort{
									Name:          "expose",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	deployment.Name = lrp.Name
	deployment.Namespace = namespace
	deployment.Spec.Template.Labels = map[string]string{
		"name": lrp.Name,
	}

	deployment.Labels = map[string]string{
		"eirini": "eirini",
		"name":   lrp.Name,
	}

	deployment.Annotations = lrp.Metadata

	return deployment
}
