package route_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	ext "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"

	"github.com/julz/cube"
	. "github.com/julz/cube/route"
	"github.com/julz/cube/route/routefakes"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Collector", func() {

	Describe("RouteCollector", func() {
		Context("Start collecting routes", func() {

			var (
				collector   *RouteCollector
				fakeClient  kubernetes.Interface
				scheduler   *routefakes.FakeTaskScheduler
				workChannel chan []RegistryMessage
				routes      []string
				host        string
				serviceName string
			)

			const (
				appName   = "dora"
				namespace = "testing"
				httpPort  = 80
				tlsPort   = 443
			)

			// handcraft json in order not to mirror the production implementation
			asJsonArray := func(uris []string) string {
				quotedUris := []string{}
				for _, uri := range uris {
					quotedUris = append(quotedUris, fmt.Sprintf("\"%s\"", uri))
				}

				return fmt.Sprintf("[%s]", strings.Join(quotedUris, ","))
			}

			createService := func(appName string) *v1.Service {
				service := &v1.Service{}

				service.Name = serviceName
				service.Namespace = namespace

				service.Annotations = map[string]string{
					"routes": asJsonArray(routes),
				}

				return service
			}

			createIngress := func(serviceName string) *ext.Ingress {
				rule := ext.IngressRule{
					Host: host,
					IngressRuleValue: ext.IngressRuleValue{
						HTTP: &ext.HTTPIngressRuleValue{
							Paths: []ext.HTTPIngressPath{
								ext.HTTPIngressPath{
									Backend: ext.IngressBackend{
										ServiceName: serviceName,
									},
								},
							},
						},
					},
				}

				return &ext.Ingress{
					Spec: ext.IngressSpec{
						Rules: []ext.IngressRule{rule},
					},
				}
			}

			createFakes := func() {
				ingress := createIngress(serviceName)
				service := createService(appName)

				_, err := fakeClient.CoreV1().Services(namespace).Create(service)
				Expect(err).ToNot(HaveOccurred())

				_, err = fakeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
				Expect(err).ToNot(HaveOccurred())
			}

			BeforeEach(func() {
				serviceName = cube.GetInternalServiceName(appName)
				host = fmt.Sprintf("%s.%s", serviceName, "kube-endpoint")
				routes = []string{"route1.app.com", "route2.app.com"}

				scheduler = new(routefakes.FakeTaskScheduler)
				workChannel = make(chan []RegistryMessage, 1)
				fakeClient = fake.NewSimpleClientset()

				createFakes()
			})

			JustBeforeEach(func() {
				collector = &RouteCollector{
					Client:        fakeClient,
					Scheduler:     scheduler,
					Work:          workChannel,
					KubeNamespace: namespace,
				}

				collector.Start()
				task := scheduler.ScheduleArgsForCall(0)
				err := task()
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use the scheduler to collect routes", func() {
				Expect(scheduler.ScheduleCallCount()).To(Equal(1))
			})

			It("should send the correct RegistryMessage in the work channel", func() {
				actualMessages := <-workChannel
				expectedMessages := []RegistryMessage{
					RegistryMessage{
						Host:    host,
						URIs:    routes,
						Port:    httpPort,
						TlsPort: tlsPort,
						App:     serviceName,
					},
				}
				Expect(actualMessages).To(Equal(expectedMessages))
			})
		})

	})

})
