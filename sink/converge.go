package sink

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube"
	"github.com/julz/cube/opi"
)

type Converger struct {
	Converter   Converter
	Desirer     opi.Desirer
	CfClient    cube.CfClient
	Client      *http.Client
	Logger      lager.Logger
	RegistryUrl string
	RegistryIP  string
}

// this is a brain-dead simple initial implementation, obviously
// it could be optimised by checking for which things changed etc,
// and you could avoid using an array with a channel of fingerprints but..
// honestly for any reasonable number of apps, memory is cheap and
// any efficiency is likely dominated by the actual business of actioning
// the requests
func (c *Converger) ConvergeOnce(ctx context.Context, ccMessages []cc_messages.DesireAppRequestFromCC) error {
	desire := make([]opi.LRP, 0)
	for _, msg := range ccMessages {
		lrp := c.convertMessage(msg)
		desire = append(desire, lrp)
	}
	return c.Desirer.Desire(ctx, desire)
}

// Convert could panic. To be able to skip this message and continue with the next,
// the panic needs to be handled for each message.
func (c *Converger) convertMessage(msg cc_messages.DesireAppRequestFromCC) opi.LRP {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				c.Logger.Error("failed-to-convert-message", err)
			}
		}
	}()
	return c.Converter.Convert(msg, c.RegistryUrl, c.RegistryIP, c.CfClient, c.Client, c.Logger)
}

type Converter interface {
	Convert(cc cc_messages.DesireAppRequestFromCC, registryUrl string, registryIP string, cfClient cube.CfClient, client *http.Client, log lager.Logger) opi.LRP
}

type ConvertFunc func(cc cc_messages.DesireAppRequestFromCC, registryUrl string, registryIP string, cfClient cube.CfClient, client *http.Client, log lager.Logger) opi.LRP

func (fn ConvertFunc) Convert(cc cc_messages.DesireAppRequestFromCC, registryUrl string, registryIP string, cfClient cube.CfClient, client *http.Client, log lager.Logger) opi.LRP {
	return fn(cc, registryUrl, registryIP, cfClient, client, log)
}
