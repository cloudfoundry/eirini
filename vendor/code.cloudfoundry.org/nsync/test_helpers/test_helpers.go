package test_helpers

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"github.com/cloudfoundry-incubator/routing-info/cfroutes"
	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
	. "github.com/onsi/gomega"
)

func VerifyHttpRoutes(routes models.Routes, expectedCfRoutes cfroutes.CFRoutes) {
	cfRoutes := cfroutes.CFRoutes{}
	jsonPayload, err := routes[cfroutes.CF_ROUTER].MarshalJSON()
	Expect(err).NotTo(HaveOccurred())
	err = json.Unmarshal(jsonPayload, &cfRoutes)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfRoutes).To(ConsistOf(expectedCfRoutes))
}

func VerifyTcpRoutes(routes models.Routes, expectedTcpRoutes tcp_routes.TCPRoutes) {
	tcpRoutes := tcp_routes.TCPRoutes{}
	jsonPayload, err := routes[tcp_routes.TCP_ROUTER].MarshalJSON()
	Expect(err).NotTo(HaveOccurred())
	err = json.Unmarshal(jsonPayload, &tcpRoutes)
	Expect(err).NotTo(HaveOccurred())
	Expect(tcpRoutes).To(ConsistOf(expectedTcpRoutes))
}
