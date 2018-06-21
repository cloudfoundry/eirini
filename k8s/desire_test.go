package k8s_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
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
		client            *kubernetes.Clientset
		ingressManager    *k8sfakes.FakeIngressManager
		deploymentManager *k8sfakes.FakeDeploymentManager
		desirer           *k8s.Desirer
		namespace         string
		lrps              []opi.LRP
		vcapAppNames      []string
		lrpUris           [][]string
	)

	const timeout time.Duration = 60 * time.Second

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

	// handcraft json in order not to mirror the production implementation
	asJsonArray := func(uris []string) string {
		quotedUris := []string{}
		for _, uri := range uris {
			quotedUris = append(quotedUris, fmt.Sprintf("\"%s\"", uri))
		}

		return fmt.Sprintf("[%s]", strings.Join(quotedUris, ","))
	}

	metaFor := func(uris []string) map[string]string {
		jsonUris := asJsonArray(uris)
		return map[string]string{
			cf.VcapAppUris: fmt.Sprintf("%s", jsonUris),
		}
	}

	cleanupDeployment := func(appName string) {
		if err := client.AppsV1beta1().Deployments(namespace).Delete(appName, &metav1.DeleteOptions{}); err != nil {
			panic(err)
		}
	}

	cleanupService := func(appName string) {
		serviceName := eirini.GetInternalServiceName(appName)
		if err := client.CoreV1().Services(namespace).Delete(serviceName, &metav1.DeleteOptions{}); err != nil {
			panic(err)
		}
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
		vcapAppNames = []string{"vcap-app-name0", "vcap-app-name1"}

		lrpUris = [][]string{
			[]string{"https://app-0.eirini.cf/", "https://commahere.eirini.cf/,,"},
			[]string{"https://app-1.eirini.cf/", "https://commahere.eirini.cf/,,"},
		}

		lrps = []opi.LRP{
			{Name: "app0", Image: "busybox", TargetInstances: 1, Command: []string{""}, Metadata: metaFor(lrpUris[0])},
			{Name: "app1", Image: "busybox", TargetInstances: 3, Command: []string{""}, Metadata: metaFor(lrpUris[1])},
		}

		ingressManager = new(k8sfakes.FakeIngressManager)
		deploymentManager = new(k8sfakes.FakeDeploymentManager)
	})

	JustBeforeEach(func() {
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		desirer = k8s.NewDesirer(client, namespace, ingressManager, deploymentManager)
	})

	Context("When a LRP is desired", func() {

		getDeploymentNames := func(deployments *v1beta1.DeploymentList) []string {
			depNames := []string{}
			for _, deployment := range deployments.Items {
				depNames = append(depNames, deployment.ObjectMeta.Name)
			}

			return depNames
		}

		verifyUpdateIngressArgsForCall := func(i int) {
			actualNamespace, actualLrp := ingressManager.UpdateIngressArgsForCall(i)

			Expect(actualNamespace).To(Equal(namespace))
			Expect(actualLrp).To(Equal(lrps[i]))
		}

		Context("When it succeeds", func() {

			AfterEach(func() {
				for _, appName := range getLRPNames() {
					cleanupDeployment(appName)
					cleanupService(appName)
				}

				Eventually(listDeployments, timeout).Should(BeEmpty())
				Eventually(listServices, timeout).Should(BeEmpty())
			})

			It("Creates deployments for every LRP in the array", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

				deployments, err := client.AppsV1beta1().Deployments(namespace).List(av1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(deployments.Items, timeout).Should(HaveLen(len(lrps)))
				Expect(getDeploymentNames(deployments)).To(ConsistOf(getLRPNames()))
			})

			It("Creates services for every deployment", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

				services, err := client.CoreV1().Services(namespace).List(av1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Eventually(services.Items, timeout).Should(HaveLen(len(lrps)))
			})

			It("Should store URIs", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())
				services, err := client.CoreV1().Services(namespace).List(av1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				for i, service := range services.Items {
					expectedVcapApp := asJsonArray(lrpUris[i])
					Expect(service.Annotations["routes"]).To(Equal(expectedVcapApp))
				}
			})

			// TODO: Currently mixture of integration and unit tests.
			// It should be extracted into a separate unit-test suite.
			It("Adds an ingress rule for each app", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

				Eventually(ingressManager.UpdateIngressCallCount(), timeout).Should(Equal(len(lrps)))
				for i := 0; i < len(lrps); i++ {
					verifyUpdateIngressArgsForCall(i)
				}
			})

			It("Doesn't error when the deployment already exists", func() {
				for i := 0; i < 2; i++ {
					Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())
				}
			})
		})

		// TODO: Currently mixture of integration and unit tests.
		// It should be extracted into a separate unit-test suite.
		Context("When the IngressManager failes to update", func() {

			var (
				expectedErr error
				lrp         opi.LRP
			)

			BeforeEach(func() {
				lrp = lrps[0]
				expectedErr = errors.New("failed to update ingress")
				ingressManager.UpdateIngressReturns(expectedErr)
			})

			It("Propagates the error", func() {
				actualErr := desirer.Desire(context.Background(), []opi.LRP{lrp})
				Expect(actualErr).To(Equal(expectedErr))
			})

			AfterEach(func() {
				cleanupDeployment(lrp.Name)
				Eventually(listDeployments, timeout).Should(BeEmpty())

				cleanupService(lrp.Name)
				Eventually(listServices, timeout).Should(BeEmpty())
			})

		})

	})

	createDeployment := func(appName, image string, replicas int32, command []string) *v1beta1.Deployment {
		deployment := &v1beta1.Deployment{
			Spec: v1beta1.DeploymentSpec{
				Replicas: &replicas,
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							v1.Container{
								Name:    "cont",
								Image:   image,
								Command: command,
								Env:     []v1.EnvVar{v1.EnvVar{Name: "GOPATH", Value: "~/go"}},
							},
						},
					},
				},
			},
		}

		deployment.Name = appName
		deployment.Annotations = map[string]string{cf.LastUpdated: "whenever"}
		deployment.Spec.Template.Labels = map[string]string{
			"name": appName,
		}

		return deployment
	}

	Context("List LRPs", func() {

		var (
			err        error
			actualLRPs []opi.LRP
		)

		JustBeforeEach(func() {
			actualLRPs, err = desirer.List(context.Background())
		})

		Context("When list is succesful", func() {

			var lrps []opi.LRP

			BeforeEach(func() {
				lrps = []opi.LRP{
					opi.LRP{Name: "missing", Image: "docker.png"},
					opi.LRP{Name: "wanted", Image: "docker.jpg"},
				}

				deploymentManager.ListLRPsReturns(lrps, nil)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should delegate to DeploymentManager", func() {
				Expect(deploymentManager.ListLRPsCallCount()).To(Equal(1))
				Expect(deploymentManager.ListLRPsArgsForCall(0)).To(Equal(namespace))
			})

			It("propagates the returned LRPs from DeploymentManager", func() {
				Expect(actualLRPs).To(Equal(lrps))
			})
		})

		Context("When list fails", func() {

			BeforeEach(func() {
				deploymentManager.ListLRPsReturns([]opi.LRP{}, errors.New("failed-to-list-lrps"))
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Get LRP by name", func() {

		var (
			appName  string
			image    string
			command  []string
			replicas int32
			lrp      *opi.LRP
			err      error
		)

		Context("When it exists", func() {
			BeforeEach(func() {
				appName = "test-app"
				image = "busybox"
				command = []string{"ls", "-la"}
				replicas = int32(2)

				expectedDep := createDeployment(appName, image, replicas, command)
				_, err := client.AppsV1beta1().Deployments(namespace).Create(expectedDep)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				lrp, err = desirer.Get(context.Background(), appName)
			})

			It("should return the correct LRP", func() {
				Expect(lrp.Name).To(Equal(appName))
				Expect(lrp.Image).To(Equal(image))
				Expect(lrp.Command).To(Equal(command))
				Expect(lrp.Env).To(Equal(map[string]string{"GOPATH": "~/go"}))
				Expect(lrp.TargetInstances).To(Equal(int(replicas)))
			})

			It("should contain last_updated timestamp", func() {
				Expect(lrp.Metadata[cf.LastUpdated]).To(Equal("whenever"))
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				cleanupDeployment("test-app")
				Eventually(listDeployments, timeout).Should(BeEmpty())
			})
		})

		Context("when it does not exist", func() {

			var (
				lrp *opi.LRP
				err error
			)

			JustBeforeEach(func() {
				lrp, err = desirer.Get(context.Background(), "test-app")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not return a LRP", func() {
				Expect(lrp).To(BeNil())
			})

		})
	})

	Context("Update an app", func() {
		Context("when the app exists", func() {

			var (
				appName  string
				image    string
				command  []string
				replicas int32
			)

			BeforeEach(func() {
				appName = "test-app"
				image = "busybox"
				command = []string{"ls", "-la"}
				replicas = int32(2)

				expectedDep := createDeployment(appName, image, replicas, command)
				_, err := client.AppsV1beta1().Deployments(namespace).Create(expectedDep)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with replica count modified", func() {

				var err error

				getDeployment := func(appName string) *v1beta1.Deployment {
					dep, err := client.AppsV1beta1().Deployments(namespace).Get(appName, av1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return dep
				}

				JustBeforeEach(func() {
					err = desirer.Update(context.Background(), opi.LRP{
						Name:            appName,
						TargetInstances: 5,
						Metadata:        map[string]string{cf.LastUpdated: "123214.2"}})
				})

				It("scales the app without error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("updates the desired number of app instances", func() {
					Eventually(func() int32 {
						return *getDeployment(appName).Spec.Replicas
					}, timeout).Should(Equal(int32(5)))
				})

				It("creates the desired number of app instances", func() {
					Eventually(func() int32 {
						return getDeployment(appName).Status.Replicas
					}, timeout).Should(Equal(int32(5)))

					Eventually(func() int32 {
						return getDeployment(appName).Status.UpdatedReplicas
					}, timeout).Should(Equal(int32(5)))
				})

				It("updates the timestamp of the last update", func() {
					Eventually(func() map[string]string {
						return getDeployment(appName).Annotations
					}, timeout).Should(HaveKeyWithValue(cf.LastUpdated, "123214.2"))
				})

				AfterEach(func() {
					cleanupDeployment(appName)
					Eventually(listDeployments, timeout).Should(BeEmpty())
				})
			})
		})

		Context("when the app does not exist", func() {

			var (
				err     error
				appName string
			)

			JustBeforeEach(func() {
				err = desirer.Update(context.Background(), opi.LRP{Name: appName, TargetInstances: 2})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not create the app", func() {
				_, err := client.AppsV1beta1().Deployments(namespace).Get(appName, av1.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
