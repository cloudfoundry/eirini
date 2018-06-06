package bifrost

import (
	"context"
	"net/http"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type Bifrost struct {
	Converter   Converter
	Desirer     opi.Desirer
	CfClient    eirini.CfClient
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
func (c *Bifrost) Transfer(ctx context.Context, ccMessages []cc_messages.DesireAppRequestFromCC) error {
	desire := make([]opi.LRP, 0)
	for _, msg := range ccMessages {
		lrp := c.convertMessage(msg)
		desire = append(desire, lrp)
	}
	return c.Desirer.Desire(ctx, desire)
}

// Convert could panic. To be able to skip this message and continue with the next,
// the panic needs to be handled for each message.
func (c *Bifrost) convertMessage(msg cc_messages.DesireAppRequestFromCC) opi.LRP {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				c.Logger.Error("failed-to-convert-message", err)
			}
		}
	}()
	return c.Converter.Convert(msg, c.RegistryUrl, c.RegistryIP, c.CfClient, c.Client, c.Logger)
}

func (b *Bifrost) List(ctx context.Context) ([]models.DesiredLRPSchedulingInfo, error) {
	lrps, err := b.Desirer.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list desired LRPs")
	}

	infos := toDesiredLRPSchedulingInfo(lrps)

	return infos, nil
}

func toDesiredLRPSchedulingInfo(lrps []opi.LRP) []models.DesiredLRPSchedulingInfo {
	infos := []models.DesiredLRPSchedulingInfo{}
	for _, l := range lrps {
		info := models.DesiredLRPSchedulingInfo{}
		info.ProcessGuid = l.Name
		infos = append(infos, info)
	}
	return infos
}
