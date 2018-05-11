package k8s_test

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	"github.com/julz/cube"
	. "github.com/julz/cube/k8s"
	"github.com/julz/cube/opi"

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

	Context("Create Ingress", func() {

		Context("When successfully created", func() {

			var (
				createdIngress *ext.Ingress
				err            error
			)

			assertIngress := func(ingress *ext.Ingress) {
				Expect(ingress.Name).To(Equal(ingressName))
				Expect(ingress.Namespace).To(Equal(namespace))
				Expect(ingress.Kind).To(Equal(ingressKind))
				Expect(ingress.APIVersion).To(Equal(ingressAPIVersion))
				Expect(ingress.Spec.TLS).To(HaveLen(1))
				Expect(ingress.Spec.TLS[0].Hosts).To(HaveLen(0))
			}

			JustBeforeEach(func() {
				createdIngress, err = ingressManager.CreateIngress(namespace)
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("shoud return expected object", func() {
				assertIngress(createdIngress)
			})

			It("should create the expected object in k8s", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get("eirini", av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				assertIngress(ingress)
			})
		})
	})

	Context("Update Ingress", func() {

		var err error

		const (
			ingressAppName = "app-name"
			lrpName        = "new-app-name"
			vcapName       = "new-vcaapp-name"
		)

		createIngressRule := func(serviceName string) ext.IngressRule {
			ingress := ext.IngressRule{
				Host: fmt.Sprintf("%s.%s", vcapName, kubeEndpoint),
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

			rule := createIngressRule(cube.GetInternalServiceName(ingressAppName))
			ingress.Spec.Rules = append(ingress.Spec.Rules, rule)

			return ingress
		}

		JustBeforeEach(func() {
			lrp := opi.LRP{Name: lrpName}
			vcap := VcapApp{AppName: vcapName}

			err = ingressManager.UpdateIngress(namespace, lrp, vcap)
		})

		Context("When ingress already exists", func() {

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

				tlsHosts := []string{fmt.Sprintf("%s.%s", ingressAppName, kubeEndpoint), fmt.Sprintf("%s.%s", vcapName, kubeEndpoint)}
				Expect(ingress.Spec.TLS).To(Equal([]ext.IngressTLS{
					ext.IngressTLS{
						Hosts: tlsHosts,
					},
				}))

				Expect(ingress.Spec.Rules).To(Equal([]ext.IngressRule{
					createIngressRule(cube.GetInternalServiceName(ingressAppName)),
					createIngressRule(cube.GetInternalServiceName(lrpName)),
				}))
			})

		})

		Context("When ingress does not exist", func() {

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should update the record", func() {
				ingress, err := fakeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				tlsHosts := []string{fmt.Sprintf("%s.%s", vcapName, kubeEndpoint)}
				Expect(ingress.Spec.TLS).To(Equal([]ext.IngressTLS{
					ext.IngressTLS{
						Hosts: tlsHosts,
					},
				}))

				Expect(ingress.Spec.Rules).To(Equal([]ext.IngressRule{
					createIngressRule(cube.GetInternalServiceName(lrpName)),
				}))
			})

		})

	})
})
