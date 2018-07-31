package k8s_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
)

var _ = Describe("Desire", func() {
	var (
		desirer         *Desirer
		instanceManager *k8sfakes.FakeInstanceManager
		ingressManager  *k8sfakes.FakeIngressManager
		serviceManager  *k8sfakes.FakeServiceManager

		namespace string
	)

	BeforeEach(func() {
		namespace = "asgard"
		instanceManager = new(k8sfakes.FakeInstanceManager)
		ingressManager = new(k8sfakes.FakeIngressManager)
		serviceManager = new(k8sfakes.FakeServiceManager)
	})

	JustBeforeEach(func() {
		desirer = NewTestDesirer(instanceManager, ingressManager, serviceManager)
	})

	Context("When desiring an lrp", func() {
		var (
			err error
			lrp *opi.LRP
		)
		BeforeEach(func() {
			lrp = &opi.LRP{}
		})

		JustBeforeEach(func() {
			err = desirer.Desire(lrp)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the instance manager list function", func() {
			count := instanceManager.ExistsCallCount()
			Expect(count).To(Equal(1))
		})

		It("should call the instance manager create function", func() {
			count := instanceManager.CreateCallCount()
			Expect(count).To(Equal(1))

			actualLRP := instanceManager.CreateArgsForCall(0)
			Expect(actualLRP).To(Equal(lrp))
		})

		It("should call the service manager create function", func() {
			count := serviceManager.CreateCallCount()
			Expect(count).To(Equal(1))

			actualLRP := serviceManager.CreateArgsForCall(0)
			Expect(actualLRP).To(Equal(lrp))
		})

		It("should create the ingress using the ingress manager", func() {
			count := ingressManager.UpdateCallCount()
			Expect(count).To(Equal(1))

			actualLRP := ingressManager.UpdateArgsForCall(0)
			Expect(actualLRP).To(Equal(lrp))
		})

		Context("When desiring a LRP fails", func() {

			Context("When the instance manager is failing on exists", func() {
				BeforeEach(func() {
					instanceManager.ExistsReturns(false, errors.New("argh!"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("shouldn't interact with any other managers", func() {
					Expect(instanceManager.CreateCallCount()).To(Equal(0))
					Expect(ingressManager.UpdateCallCount()).To(Equal(0))
					Expect(serviceManager.CreateCallCount()).To(Equal(0))
				})
			})

			Context("When the instance manager is failing on create", func() {
				BeforeEach(func() {
					instanceManager.CreateReturns(errors.New("argh!"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("shouldn't interact with any other managers", func() {
					Expect(ingressManager.UpdateCallCount()).To(Equal(0))
					Expect(serviceManager.CreateCallCount()).To(Equal(0))
				})
			})

			Context("When the service manager fails", func() {
				BeforeEach(func() {
					serviceManager.CreateReturns(errors.New("argh!"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("shouldn't interact with the ingress manager", func() {
					Expect(ingressManager.UpdateCallCount()).To(Equal(0))
				})
			})

			Context("When update ingress fails", func() {
				BeforeEach(func() {
					ingressManager.UpdateReturns(errors.New("argh!"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("When listing actual LRPs", func() {
			var (
				err        error
				actualLRPs []*opi.LRP
				lrps       []*opi.LRP
			)

			BeforeEach(func() {
				lrps = []*opi.LRP{&opi.LRP{Name: "app1"}, &opi.LRP{Name: "app2"}}
				instanceManager.ListReturns(lrps, nil)
			})

			JustBeforeEach(func() {
				actualLRPs, err = desirer.List()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the actual lrps", func() {
				Expect(actualLRPs).To(Equal(lrps))
			})

			Context("When listing acutial LRP fails", func() {
				BeforeEach(func() {
					instanceManager.ListReturns(nil, errors.New("buuh"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("When getting an LRP", func() {

			var (
				name      string
				lrp       *opi.LRP
				actualLRP *opi.LRP
			)

			BeforeEach(func() {
				name = "lothbrok"
				lrp = &opi.LRP{}
				instanceManager.GetReturns(lrp, nil)
			})

			JustBeforeEach(func() {
				actualLRP, err = desirer.Get(name)
			})

			It("should not fail", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the correct lrp", func() {
				Expect(actualLRP).To(Equal(lrp))
			})

			It("should use the instance manager", func() {
				Expect(instanceManager.GetCallCount()).To(Equal(1))
				actualName := instanceManager.GetArgsForCall(0)
				Expect(actualName).To(Equal(name))
			})

			Context("When the instance manager errors", func() {
				BeforeEach(func() {
					instanceManager.GetReturns(nil, errors.New("failed-to-get-lrp"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("When updating an LRP", func() {

			var (
				lrp *opi.LRP
			)

			BeforeEach(func() {
				lrp = &opi.LRP{}
			})

			JustBeforeEach(func() {
				err = desirer.Update(lrp)
			})

			It("should not fail", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should update the expected LRP", func() {
				updateLrp := instanceManager.UpdateArgsForCall(0)
				Expect(updateLrp).To(Equal(lrp))
			})

			Context("When updating an LRP fails", func() {

				BeforeEach(func() {
					instanceManager.UpdateReturns(errors.New("doing"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("When stopping an lrp", func() {
			var name string

			BeforeEach(func() {
				name = "loki"
			})

			JustBeforeEach(func() {
				err = desirer.Stop(name)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use the instance manager to deletes the lrp", func() {
				Expect(instanceManager.DeleteCallCount()).To(Equal(1))
				actualName := instanceManager.DeleteArgsForCall(0)
				Expect(actualName).To(Equal(name))
			})

			It("should use the ingress manager to delete the ingress", func() {
				Expect(ingressManager.DeleteCallCount()).To(Equal(1))
				actualName := ingressManager.DeleteArgsForCall(0)
				Expect(actualName).To(Equal(name))
			})

			It("should use the service manager to delete the service", func() {
				Expect(serviceManager.DeleteCallCount()).To(Equal(1))
				actualName := serviceManager.DeleteArgsForCall(0)
				Expect(actualName).To(Equal(name))
			})

			Context("When deleting a LRP fails", func() {

				BeforeEach(func() {
					instanceManager.DeleteReturns(errors.New("crash"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should not use any of the other managers", func() {
					Expect(ingressManager.DeleteCallCount()).To(Equal(0))
					Expect(serviceManager.DeleteCallCount()).To(Equal(0))
				})
			})

			Context("When deleting an ingress fails", func() {

				BeforeEach(func() {
					ingressManager.DeleteReturns(errors.New("crash"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should not use the service manager", func() {
					Expect(serviceManager.DeleteCallCount()).To(Equal(0))
				})
			})

			Context("When deleting an ingress fails", func() {

				BeforeEach(func() {
					serviceManager.DeleteReturns(errors.New("crash"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
