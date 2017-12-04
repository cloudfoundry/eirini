package internalroutes

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
)

const INTERNAL_ROUTER = "internal-router"

type InternalRoutes []InternalRoute

type InternalRoute struct {
	Hostname string `json:"hostname"`
}

func (c InternalRoutes) RoutingInfo() models.Routes {
	data, _ := json.Marshal(c)
	routingInfo := json.RawMessage(data)
	return models.Routes{
		INTERNAL_ROUTER: &routingInfo,
	}
}

func InternalRoutesFromRoutingInfo(routingInfo models.Routes) (InternalRoutes, error) {
	if routingInfo == nil {
		return nil, nil
	}

	routes := routingInfo
	data, found := routes[INTERNAL_ROUTER]
	if !found {
		return nil, nil
	}

	if data == nil {
		return nil, nil
	}

	internalRoutes := InternalRoutes{}
	err := json.Unmarshal(*data, &internalRoutes)

	return internalRoutes, err
}
