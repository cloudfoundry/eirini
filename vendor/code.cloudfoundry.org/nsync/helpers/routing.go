package helpers

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/cloudfoundry-incubator/routing-info/cfroutes"
	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
)

type routingKey struct {
	Port            uint32
	RouteServiceUrl string
}

func CCRouteInfoToRoutes(ccRoutes cc_messages.CCRouteInfo, ports []uint32) (models.Routes, error) {
	var defaultPort uint32
	if len(ports) > 0 {
		defaultPort = ports[0]
	} else {
		defaultPort = 8080
	}

	routes := models.Routes{}

	if ccRoutes[cc_messages.CC_HTTP_ROUTES] != nil {
		httpRoutingInfo, err := constructHttpRoutes(ccRoutes, defaultPort)
		if err != nil {
			return nil, err
		}
		routes[cfroutes.CF_ROUTER] = httpRoutingInfo[cfroutes.CF_ROUTER]
	} else {
		cfRoutes := cfroutes.CFRoutes{}
		httpRoutingInfo := cfRoutes.RoutingInfo()
		routes[cfroutes.CF_ROUTER] = httpRoutingInfo[cfroutes.CF_ROUTER]
	}

	if ccRoutes[cc_messages.CC_TCP_ROUTES] != nil {
		tcpRoutingInfo, err := constructTcpRoutes(ccRoutes)
		if err != nil {
			return nil, err
		}
		routes[tcp_routes.TCP_ROUTER] = tcpRoutingInfo[tcp_routes.TCP_ROUTER]
	} else {
		tcpRoutes := tcp_routes.TCPRoutes{}
		tcpRoutingInfo := tcpRoutes.RoutingInfo()
		routes[tcp_routes.TCP_ROUTER] = (*tcpRoutingInfo)[tcp_routes.TCP_ROUTER]
	}

	return routes, nil
}

func constructTcpRoutes(ccRoutes cc_messages.CCRouteInfo) (models.Routes, error) {
	var ccTcpRoutes cc_messages.CCTCPRoutes
	err := json.Unmarshal(*ccRoutes[cc_messages.CC_TCP_ROUTES], &ccTcpRoutes)
	if err != nil {
		return nil, err
	}
	tcpRoutes := tcp_routes.TCPRoutes{}
	for _, tcpRoute := range ccTcpRoutes {
		tcpRoutes = append(tcpRoutes, tcp_routes.TCPRoute{
			RouterGroupGuid: tcpRoute.RouterGroupGuid,
			ExternalPort:    tcpRoute.ExternalPort,
			ContainerPort:   tcpRoute.ContainerPort,
		})
	}

	tcpRoutingInfoPtr := tcpRoutes.RoutingInfo()
	tcpRoutingInfo := *tcpRoutingInfoPtr
	return tcpRoutingInfo, nil
}

func constructHttpRoutes(ccRoutes cc_messages.CCRouteInfo, defaultPort uint32) (models.Routes, error) {
	var httpRoutes cc_messages.CCHTTPRoutes
	cfRoutes := make(cfroutes.CFRoutes, 0)
	routeServiceMap := make(map[routingKey][]string)

	err := json.Unmarshal(*ccRoutes[cc_messages.CC_HTTP_ROUTES], &httpRoutes)
	if err != nil {
		return nil, err
	}

	for _, httpRoute := range httpRoutes {
		key := routingKey{Port: httpRoute.Port, RouteServiceUrl: httpRoute.RouteServiceUrl}
		if key.Port == 0 {
			key.Port = defaultPort
		}
		list := routeServiceMap[key]
		routeServiceMap[key] = append(list, httpRoute.Hostname)
	}

	for key, hostnames := range routeServiceMap {
		cfRoutes = append(cfRoutes, cfroutes.CFRoute{
			Hostnames: hostnames, Port: key.Port, RouteServiceUrl: key.RouteServiceUrl,
		})
	}

	httpRoutingInfo := cfRoutes.RoutingInfo()
	return httpRoutingInfo, nil
}
