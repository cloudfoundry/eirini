package sink

import (
	"context"

	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube/opi"
)

type Converger struct {
	Converter Converter
	Desirer   opi.Desirer
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
		lrp := c.Converter.Convert(msg)
		desire = append(desire, lrp)
	}

	return c.Desirer.Desire(ctx, desire)
}

type Converter interface {
	Convert(cc cc_messages.DesireAppRequestFromCC) opi.LRP
}

type ConvertFunc func(cc cc_messages.DesireAppRequestFromCC) opi.LRP

func (fn ConvertFunc) Convert(cc cc_messages.DesireAppRequestFromCC) opi.LRP {
	return fn(cc)
}
