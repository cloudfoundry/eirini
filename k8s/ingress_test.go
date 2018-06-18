package k8s_test

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ext "k8s.io/api/extensions/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Ingress", func() {

	var (
		fakeClient     kubernetes.Interface
		ingressManager IngressManager
	)

	const (
		namespace         = "testing"
		kubeEndpoint      = "alfheim"
		ingressName       = "eirini"
		ingressKind       = "Ingress"
		ingressAPIVersion = "extensions/v1beta1"
	)

	BeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
	})

	JustBeforeEach(func() {
		ingressManager = NewIngressManager(fakeClient, kubeEndpoint)
	})

	Context("Update Ingress Rule", func() {

		var err error

		const (
			ingressAppName = "app-name"
			lrpName        = "new-app-name"
			appName        = "new-vcaapp-name"
		)

		createIngressRule := func(serviceName string) ext.IngressRule {
			ingress := ext.IngressRule{
				Host: fmt.Sprintf("%s.%s", appName, kubeEndpoint),
			}

			ingress.HTTP = &ext.HTTPIngressRuleValue{
				Paths: []ext.HTTPIngressPath{
					ext.HTTPIngressPath{
						Path: "/",
						Backend: ext.IngressBackend{
							ServiceName: serviceName,
							ServicePort: intstr.FromInt(8080),
						},
					},
				},
			}

			return ingress
		}

		createIngress := func(name, namespace string) *ext.Ingress {
			ingress := &ext.Ingress{}

			ingress.APIVersion = ingressAPIVersion
			ingress.Kind = ingressKind
			ingress.Name = name
			ingress.Namespace = namespace

			ingress.Spec.TLS = []ext.IngressTLS{
				ext.IngressTLS{
					Hosts: []string{fmt.Sprintf("%s.%s", ingressAppName, kubeEndpoint)},
				},
			}

			rule := createIngressRule(eirini.GetInternalServiceName(ingressAppName))
			ingress.Spec.Rules = append(ingress.Spec.Rules, rule)

			return ingress
		}

		JustBeforeEach(func() {
			lrp := opi.LRP{
				Name: lrpName,
				Metadata: map[string]string{
					"application_name": appName,
				}}

			err = ingressManager.UpdateIngress(namespace, lrp)
		})

		verifyTlsHosts := func(tlsHosts []string) {
			ingress, _ := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})

			Expect(ingress.Spec.TLS).To(Equal([]ext.IngressTLS{
				ext.IngressTLS{
					Hosts: tlsHosts,
				},
			}))
		}

		Context("When no ingress exists", func() {
			It("should create a new one", func() {
				_, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should add the rule for the specific lrp", func() {
				ingress, _ := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})

				rules := ingress.Spec.Rules
				Expect(rules).To(Equal([]ext.IngressRule{
					createIngressRule(eirini.GetInternalServiceName(lrpName)),
				}))

			})

			It("should add a TLS host", func() {
				tlsHosts := []string{fmt.Sprintf("%s.%s", appName, kubeEndpoint)}
				verifyTlsHosts(tlsHosts)
			})

		})

		Context("When ingress exists", func() {

			BeforeEach(func() {
				ingress := createIngress(ingressName, namespace)
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should update the record", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				Expect(ingress.Spec.Rules).To(Equal([]ext.IngressRule{
					createIngressRule(eirini.GetInternalServiceName(ingressAppName)),
					createIngressRule(eirini.GetInternalServiceName(lrpName)),
				}))
			})

			It("should add a TLS host", func() {
				tlsHosts := []string{fmt.Sprintf("%s.%s", ingressAppName, kubeEndpoint), fmt.Sprintf("%s.%s", appName, kubeEndpoint)}

				verifyTlsHosts(tlsHosts)
			})

		})
	})
})
