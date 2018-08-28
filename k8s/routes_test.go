package k8s_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/route"
)

var _ = Describe("Routes", func() {
	Context("ListRoutes", func() {

		var (
			fakeClient  kubernetes.Interface
			routeLister route.Lister
			routes      []*eirini.Routes
			err         error
		)

		const kubeNamespace = "testing"

		BeforeEach(func() {
			fakeClient = fake.NewSimpleClientset()
			routeLister = NewServiceRouteLister(fakeClient, kubeNamespace)
		})

		JustBeforeEach(func() {
			routes, err = routeLister.ListRoutes()
		})

		Context("When there are existing services", func() {

			var lrp *opi.LRP

			BeforeEach(func() {
				lrp = createLRP("baldur", "54321.0", `["my.example.route"]`)
				_, err = fakeClient.CoreV1().Services(kubeNamespace).Create(toService(lrp, kubeNamespace))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the correct routes", func() {
				Expect(routes).To(HaveLen(1))
				route := routes[0]
				Expect(route.Routes).To(ContainElement("my.example.route"))
				Expect(route.Name).To(Equal(eirini.GetInternalServiceName("baldur")))
			})

			Context("When there are headless services", func() {
				BeforeEach(func() {
					_, err = fakeClient.CoreV1().Services(kubeNamespace).Create(toHeadlessService(lrp, kubeNamespace))
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return only one Routes object", func() {
					Expect(routes).To(HaveLen(1))
				})
			})

			Context("When there are non cf services", func() {
				BeforeEach(func() {
					service := &v1.Service{}
					service.Name = "some-other-service"
					_, err = fakeClient.CoreV1().Services(namespace).Create(service)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return only one Routes object", func() {
					Expect(routes).To(HaveLen(1))
				})
			})
		})
	})
})
