package bifrost

import (
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type Converter interface {
	Convert(cc cc_messages.DesireAppRequestFromCC, registryUrl string, registryIP string, cfClient eirini.CfClient, client *http.Client, log lager.Logger) opi.LRP
}

type ConvertFunc func(cc cc_messages.DesireAppRequestFromCC, registryUrl string, registryIP string, cfClient eirini.CfClient, client *http.Client, log lager.Logger) opi.LRP

func (fn ConvertFunc) Convert(cc cc_messages.DesireAppRequestFromCC, registryUrl string, registryIP string, cfClient eirini.CfClient, client *http.Client, log lager.Logger) opi.LRP {
	return fn(cc, registryUrl, registryIP, cfClient, client, log)
}
