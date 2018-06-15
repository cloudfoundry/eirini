package bifrost

import (
	"fmt"
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

func dropletDownloadUri(baseUrl string, appGuid string) string {
	return fmt.Sprintf("%s/v2/apps/%s/droplet/download", baseUrl, appGuid)
}

func registryStageUri(baseUrl string, space string, appname string, guid string) string {
	return fmt.Sprintf("%s/v2/%s/%s/blobs/?guid=%s", baseUrl, space, appname, guid)
}
