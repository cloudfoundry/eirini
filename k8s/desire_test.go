package k8s_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/julz/cube"
	"github.com/julz/cube/k8s"
	"github.com/julz/cube/k8s/k8sfakes"
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
		client         *kubernetes.Clientset
		ingressManager *k8sfakes.FakeIngressManager
		desirer        *k8s.Desirer
		namespace      string
		lrps           []opi.LRP
		vcapAppNames   []string
		lrpUris        [][]string
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

	// handcraft json in order not to mirror the production implementation
	asJsonArray := func(uris []string) string {
		quotedUris := []string{}
		for _, uri := range uris {
			quotedUris = append(quotedUris, fmt.Sprintf("\"%s\"", uri))
		}

		return fmt.Sprintf("[%s]", strings.Join(quotedUris, ","))
	}

	envFor := func(appName string, uris []string) map[string]string {
		jsonUris := asJsonArray(uris)
		return map[string]string{
			"VCAP_APPLICATION": fmt.Sprintf("{ \"application_name\": \"%s\", \"application_uris\": %s }", appName, jsonUris),
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
			{Name: "app0", Image: "busybox", TargetInstances: 1, Command: []string{""}, Env: envFor(vcapAppNames[0], lrpUris[0])},
			{Name: "app1", Image: "busybox", TargetInstances: 3, Command: []string{""}, Env: envFor(vcapAppNames[1], lrpUris[1])},
		}

		ingressManager = new(k8sfakes.FakeIngressManager)
	})

	JustBeforeEach(func() {
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		desirer = k8s.NewDesirer(client, namespace, ingressManager)
	})

	Context("When a LPP is desired", func() {

		getDeploymentNames := func(deployments *v1beta1.DeploymentList) []string {
			depNames := []string{}
			for _, deployment := range deployments.Items {
				depNames = append(depNames, deployment.ObjectMeta.Name)
			}

			return depNames
		}

		verifyUpdateIngressArgsForCall := func(i int) {
			actualNamespace, actualLrp, actualVcapApp := ingressManager.UpdateIngressArgsForCall(i)

			Expect(actualNamespace).To(Equal(namespace))
			Expect(actualLrp).To(Equal(lrps[i]))
			Expect(actualVcapApp.AppName).To(Equal(vcapAppNames[i]))
		}

		Context("When it succeeds", func() {

			AfterEach(func() {
				for _, appName := range getLRPNames() {
					if err := client.AppsV1beta1().Deployments(namespace).Delete(appName, &metav1.DeleteOptions{}); err != nil {
						panic(err)
					}

					serviceName := cube.GetInternalServiceName(appName)
					if err := client.CoreV1().Services(namespace).Delete(serviceName, &metav1.DeleteOptions{}); err != nil {
						panic(err)
					}
				}
			})

			It("Creates deployments for every LRP in the array", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

				deployments, err := client.AppsV1beta1().Deployments(namespace).List(av1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				Expect(deployments.Items).To(HaveLen(len(lrps)))
				Expect(getDeploymentNames(deployments)).To(ConsistOf(getLRPNames()))
			})

			It("Creates services for every deployment", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

				services, err := client.CoreV1().Services(namespace).List(av1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(services.Items).To(HaveLen(len(lrps)))
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

			It("Adds an ingress rule for each app", func() {
				Expect(desirer.Desire(context.Background(), lrps)).To(Succeed())

				Expect(ingressManager.UpdateIngressCallCount()).To(Equal(len(lrps)))
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

		Context("When the IngressManager failes to update", func() {

			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("failed to update ingress")
				ingressManager.UpdateIngressReturns(expectedErr)
			})

			It("Propagates the error", func() {
				actualErr := desirer.Desire(context.Background(), lrps)
				Expect(actualErr).To(Equal(expectedErr))
			})

		})

	})

	PIt("Removes any LRPs in the namespace that are no longer desired", func() {
	})

	PIt("Updates any LRPs whose etag annotation has changed", func() {
	})
})
