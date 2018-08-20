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
		ingressName       = "eirini"
		ingressKind       = "Ingress"
		ingressAPIVersion = "extensions/v1beta1"
		ingressAppName    = "app-name"
		lrpName           = "new-app-name"
		appName           = "new-vcaapp-name"
	)

	createIngressRule := func(serviceName string, appUri string) ext.IngressRule {
		ingress := ext.IngressRule{
			Host: appUri,
		}

		ingress.HTTP = &ext.HTTPIngressRuleValue{
			Paths: []ext.HTTPIngressPath{
				{
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

	newIngress := func(name string, namespace string, appNames ...string) *ext.Ingress {
		ingress := &ext.Ingress{}

		ingress.APIVersion = ingressAPIVersion
		ingress.Kind = ingressKind
		ingress.Name = name
		ingress.Namespace = namespace

		ingress.Spec.TLS = []ext.IngressTLS{
			{

				Hosts: appNames,
			},
		}

		for _, appName := range appNames {
			rule := createIngressRule(eirini.GetInternalServiceName(appName), appName)
			ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
		}

		return ingress
	}

	createFakeIngress := func(ingressName string, namespace string, serviceNames ...string) {
		ingress := newIngress(ingressName, namespace, serviceNames...)
		ingress, createErr := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
		Expect(createErr).ToNot(HaveOccurred())
	}

	BeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
	})

	JustBeforeEach(func() {
		ingressManager = NewIngressManager(fakeClient, namespace)
	})

	Context("DeleteIngressRules", func() {
		JustBeforeEach(func() {
			err = ingressManager.Delete(appName)
		})

		Context("When there is a single rule", func() {
			BeforeEach(func() {
				createFakeIngress(ingressName, namespace, appName)
			})

			It("should remove the ingress object", func() {
				list, listErr := fakeClient.ExtensionsV1beta1().Ingresses(namespace).List(av1.ListOptions{})
				Expect(listErr).ToNot(HaveOccurred())
				Expect(list.Items).To(BeNil())
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("When there is more than one rule", func() {
			BeforeEach(func() {
				createFakeIngress(ingressName, namespace, appName, "existing_app")
			})

			getRuleServiceNames := func(rules []ext.IngressRule) []string {
				result := []string{}
				for _, rule := range rules {
					result = append(result, rule.HTTP.Paths[0].Backend.ServiceName)
				}
				return result
			}

			It("should remove the rules for the service", func() {
				ingress, getErr := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				ruleServiceNames := getRuleServiceNames(ingress.Spec.Rules)

				Expect(getErr).ToNot(HaveOccurred())
				Expect(ruleServiceNames).ToNot(ContainElement(eirini.GetInternalServiceName(appName)))
				Expect(ruleServiceNames).To(ContainElement("cf-existing_app"))
				Expect(ruleServiceNames).To(HaveLen(1))
			})
		})
	})

	Context("UpdateIngress", func() {
		JustBeforeEach(func() {
			uris, marshalErr := json.Marshal(appURIs)
			Expect(marshalErr).ToNot(HaveOccurred())

			lrp := &opi.LRP{
				Name: lrpName,
				Metadata: map[string]string{
					"application_name": appName,
					"application_uris": string(uris),
				}}

			err = ingressManager.Update(lrp)
		})

		verifyTLSHosts := func(tlsHosts []string) {
			ingress, _ := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})

			Expect(ingress.Spec.TLS).To(Equal([]ext.IngressTLS{
				{
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
					_, getErr := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
					Expect(getErr).ToNot(HaveOccurred())
				})

				It("should add the rule for the specific lrp", func() {
					ingress, _ := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})

					rules := ingress.Spec.Rules
					Expect(rules).To(Equal([]ext.IngressRule{
						createIngressRule(eirini.GetInternalServiceName(lrpName), appURIs[0]),
					}))

				})

				It("should add a TLS host", func() {
					verifyTLSHosts(appURIs)
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
					ingress, getErr := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
					Expect(getErr).ToNot(HaveOccurred())

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
					_, getErr := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
					Expect(getErr).To(HaveOccurred())
				})

			})
		})

		Context("When ingress exists", func() {
			BeforeEach(func() {
				createFakeIngress(ingressName, namespace, "existing-app")
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
					verifyTLSHosts(hosts)
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
})
