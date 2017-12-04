package internalroutes_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/routing-info/internalroutes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingInfoHelpers", func() {
	var (
		route1 internalroutes.InternalRoute
		route2 internalroutes.InternalRoute
		route3 internalroutes.InternalRoute

		routes internalroutes.InternalRoutes
	)

	BeforeEach(func() {
		route1 = internalroutes.InternalRoute{
			Hostname: "foo1.example.com",
		}
		route2 = internalroutes.InternalRoute{
			Hostname: "foo2.example.com",
		}
		route3 = internalroutes.InternalRoute{
			Hostname: "foo3.example.com",
		}

		routes = internalroutes.InternalRoutes{route1, route2, route3}
	})

	Describe("RoutingInfo", func() {
		var routingInfo models.Routes

		JustBeforeEach(func() {
			routingInfo = routes.RoutingInfo()
		})

		It("wraps the serialized routes with the correct key", func() {
			expectedBytes, err := json.Marshal(routes)
			Expect(err).NotTo(HaveOccurred())

			payload, err := routingInfo[internalroutes.INTERNAL_ROUTER].MarshalJSON()
			Expect(err).NotTo(HaveOccurred())

			Expect(payload).To(MatchJSON(expectedBytes))
		})

		Context("when InternalRoutes is empty", func() {
			BeforeEach(func() {
				routes = internalroutes.InternalRoutes{}
			})

			It("marshals an empty list", func() {
				payload, err := routingInfo[internalroutes.INTERNAL_ROUTER].MarshalJSON()
				Expect(err).NotTo(HaveOccurred())

				Expect(payload).To(MatchJSON(`[]`))
			})
		})
	})

	Describe("InternalRoutesFromRoutingInfo", func() {
		var (
			routesResult    internalroutes.InternalRoutes
			conversionError error

			routingInfo models.Routes
		)

		JustBeforeEach(func() {
			routesResult, conversionError = internalroutes.InternalRoutesFromRoutingInfo(routingInfo)
		})

		Context("when internal routes are present in the routing info", func() {
			BeforeEach(func() {
				routingInfo = routes.RoutingInfo()
			})

			It("returns the routes", func() {
				Expect(routes).To(Equal(routesResult))
			})

			Context("when the internal routes are nil", func() {
				BeforeEach(func() {
					routingInfo = models.Routes{internalroutes.INTERNAL_ROUTER: nil}
				})

				It("returns nil routes", func() {
					Expect(conversionError).NotTo(HaveOccurred())
					Expect(routesResult).To(BeNil())
				})
			})
		})

		Context("when internal routes are not present in the routing info", func() {
			BeforeEach(func() {
				routingInfo = models.Routes{}
			})

			It("returns nil routes", func() {
				Expect(conversionError).NotTo(HaveOccurred())
				Expect(routesResult).To(BeNil())
			})
		})

		Context("when the routing info is nil", func() {
			BeforeEach(func() {
				routingInfo = nil
			})

			It("returns nil routes", func() {
				Expect(conversionError).NotTo(HaveOccurred())
				Expect(routesResult).To(BeNil())
			})
		})
	})
})
