package k8s_test

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	. "code.cloudfoundry.org/eirini/k8s"
)

var _ = Describe("Service", func() {

	var (
		fakeClient     kubernetes.Interface
		serviceManager ServiceManager
		lrps           []opi.LRP
	)

	const (
		namespace = "midgard"
	)

	BeforeEach(func() {
		lrps = []opi.LRP{
			createLRP("odin", "1234.5"),
			createLRP("thor", "4567.8"),
			createLRP("mimir", "9012.3"),
		}
	})

	JustBeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
		serviceManager = NewServiceManager(fakeClient)
		for _, l := range lrps {
			fakeClient.CoreV1().Services(namespace).Create(toService(l, namespace))
		}
	})

	Context("Delete a service", func() {
		It("deletes the service", func() {
			err := serviceManager.Delete("odin", namespace)
			Expect(err).ToNot(HaveOccurred())

			services, _ := fakeClient.CoreV1().Services(namespace).List(av1.ListOptions{})
			Expect(services.Items).To(HaveLen(2))
			Expect(getServicesNames(services)).To(ConsistOf(eirini.GetInternalServiceName("mimir"), eirini.GetInternalServiceName("thor")))
		})

		Context("when the service does not exist", func() {
			It("returns an error", func() {
				err := serviceManager.Delete("non-existing", namespace)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func getServicesNames(services *v1.ServiceList) []string {
	serviceNames := []string{}
	for _, s := range services.Items {
		serviceNames = append(serviceNames, s.Name)
	}
	return serviceNames
}

func toService(lrp opi.LRP, namespace string) *v1.Service {
	service := &v1.Service{}
	service.Kind = "service"
	service.Name = eirini.GetInternalServiceName(lrp.Metadata[cf.ProcessGUID])
	service.Namespace = namespace

	return service
}
