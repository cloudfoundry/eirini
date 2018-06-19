package k8s_test

import (
	"encoding/json"

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
		appURIs        []string

		err error
	)

	const (
		namespace         = "testing"
		kubeEndpoint      = "alfheim"
		ingressName       = "eirini"
		ingressKind       = "Ingress"
		ingressAPIVersion = "extensions/v1beta1"
		ingressAppName    = "app-name"
		lrpName           = "new-app-name"
		appName           = "new-vcaapp-name"
	)

	BeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
	})

	JustBeforeEach(func() {
		ingressManager = NewIngressManager(fakeClient, kubeEndpoint)
	})

	createIngressRule := func(serviceName string, appUri string) ext.IngressRule {
		ingress := ext.IngressRule{
			Host: appUri,
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

	newIngress := func(name, namespace, appName string) *ext.Ingress {
		ingress := &ext.Ingress{}

		ingress.APIVersion = ingressAPIVersion
		ingress.Kind = ingressKind
		ingress.Name = name
		ingress.Namespace = namespace

		ingress.Spec.TLS = []ext.IngressTLS{
			ext.IngressTLS{
				Hosts: []string{appName},
			},
		}

		rule := createIngressRule(eirini.GetInternalServiceName(appName), appName)
		ingress.Spec.Rules = append(ingress.Spec.Rules, rule)

		return ingress
	}

	JustBeforeEach(func() {
		uris, err := json.Marshal(appURIs)
		Expect(err).ToNot(HaveOccurred())

		lrp := opi.LRP{
			Name: lrpName,
			Metadata: map[string]string{
				"application_name": appName,
				"application_uris": string(uris),
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

	BeforeEach(func() {
		appURIs = []string{"alpha.example.com"}
	})

	Context("When no ingress exists", func() {
		Context("When an app has one route", func() {
			It("should create a new one", func() {
				_, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should add the rule for the specific lrp", func() {
				ingress, _ := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})

				rules := ingress.Spec.Rules
				Expect(rules).To(Equal([]ext.IngressRule{
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[0]),
				}))

			})

			It("should add a TLS host", func() {
				verifyTlsHosts(appURIs)
			})
		})

		Context("When an app has multiple routes", func() {
			BeforeEach(func() {
				appURIs = []string{
					"alpha.example.com",
					"beta.example.com",
					"gamma.example.com",
				}
			})

			It("should create an ingress rule for each route", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				Expect(ingress.Spec.Rules).To(Equal([]ext.IngressRule{
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[0]),
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[1]),
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[2]),
				}))
			})
		})

		Context("When an app has no routes", func() {
			BeforeEach(func() {
				appURIs = []string{}
			})

			It("shouldn't create an ingress at all", func() {
				_, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Context("When ingress exists", func() {
		BeforeEach(func() {
			ingress := newIngress(ingressName, namespace, "existing-app")
			ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("When an app has one route", func() {
			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should update the record", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				Expect(ingress.Spec.Rules).To(Equal([]ext.IngressRule{
					createIngressRule("cf-existing-app", "existing-app"),
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[0]),
				}))
			})

			It("should add a TLS host", func() {
				hosts := append([]string{"existing-app"}, appURIs...)
				verifyTlsHosts(hosts)
			})
		})

		Context("When an app has multiple routes", func() {
			BeforeEach(func() {
				appURIs = []string{
					"alpha.example.com",
					"beta.example.com",
					"gamma.example.com",
				}
			})

			It("should create an ingress rule for each route", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				Expect(ingress.Spec.Rules).To(Equal([]ext.IngressRule{
					createIngressRule("cf-existing-app", "existing-app"),
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[0]),
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[1]),
					createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[2]),
				}))
			})
		})

		Context("When an app has no routes", func() {
			BeforeEach(func() {
				appURIs = []string{}
			})

			It("shouldn't create an ingress rule", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(ingress.Spec.Rules).To(HaveLen(1))
			})
		})
	})

})
