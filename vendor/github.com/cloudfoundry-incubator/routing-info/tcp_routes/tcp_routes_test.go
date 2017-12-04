package tcp_routes_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TcpRoutes", func() {
	var (
		route1 tcp_routes.TCPRoute
		route2 tcp_routes.TCPRoute
		route3 tcp_routes.TCPRoute

		routes tcp_routes.TCPRoutes
	)

	BeforeEach(func() {
		route1 = tcp_routes.TCPRoute{
			RouterGroupGuid: "some-guid",
			ExternalPort:    5222,
			ContainerPort:   11111,
		}
		route2 = tcp_routes.TCPRoute{
			RouterGroupGuid: "some-guid-2",
			ExternalPort:    5223,
			ContainerPort:   22222,
		}
		route3 = tcp_routes.TCPRoute{
			RouterGroupGuid: "some-guid-3",
			ExternalPort:    5224,
			ContainerPort:   33333,
		}

		routes = tcp_routes.TCPRoutes{route1, route2, route3}
	})

	Describe("RoutingInfo", func() {
		var (
			routingInfoPtr *models.Routes
			routingInfo    models.Routes
		)

		JustBeforeEach(func() {
			routingInfoPtr = routes.RoutingInfo()
			Expect(routingInfoPtr).NotTo(BeNil())
			routingInfo = *routingInfoPtr
		})

		It("wraps the serialized routes with the correct key", func() {
			expectedBytes, err := json.Marshal(routes)
			Expect(err).NotTo(HaveOccurred())

			payload, err := routingInfo[tcp_routes.TCP_ROUTER].MarshalJSON()
			Expect(err).NotTo(HaveOccurred())

			Expect(payload).To(MatchJSON(expectedBytes))
		})

		Context("when TCPRoutes is empty", func() {
			BeforeEach(func() {
				routes = tcp_routes.TCPRoutes{}
			})

			It("marshals an empty list", func() {
				payload, err := routingInfo[tcp_routes.TCP_ROUTER].MarshalJSON()
				Expect(err).NotTo(HaveOccurred())

				Expect(payload).To(MatchJSON(`[]`))
			})
		})
	})

	Describe("TCPRoutesFromRoutingInfo", func() {
		var (
			routesResult    tcp_routes.TCPRoutes
			conversionError error
			routingInfo     *models.Routes
		)

		JustBeforeEach(func() {
			routesResult, conversionError = tcp_routes.TCPRoutesFromRoutingInfo(routingInfo)
		})

		Context("when TCP routes are present in the routing info", func() {
			BeforeEach(func() {
				routingInfo = routes.RoutingInfo()
			})

			It("returns the routes", func() {
				Expect(routesResult).To(Equal(routes))
			})

			Context("when the TCP routes are nil", func() {
				BeforeEach(func() {
					routingInfo = &models.Routes{tcp_routes.TCP_ROUTER: nil}
				})

				It("returns nil routes", func() {
					Expect(conversionError).NotTo(HaveOccurred())
					Expect(routesResult).To(BeNil())
				})
			})
		})

		Context("when TCP routes are not present in the routing info", func() {
			BeforeEach(func() {
				routingInfo = &models.Routes{}
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
