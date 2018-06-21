package bifrost

import (
	"context"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
)

type Bifrost struct {
	Converter Converter
	Desirer   opi.Desirer
	Logger    lager.Logger
}

func (c *Bifrost) Transfer(ctx context.Context, request cf.DesireLRPRequest) error {
	desiredLRP, err := c.Converter.Convert(request)
	if err != nil {
		c.Logger.Error("failed-to-convert-request", err, lager.Data{"desire-lrp-request": request})
		return err
	}
	return c.Desirer.Desire(ctx, []opi.LRP{desiredLRP})
}

func (b *Bifrost) List(ctx context.Context) ([]*models.DesiredLRPSchedulingInfo, error) {
	lrps, err := b.Desirer.List(ctx)
	if err != nil {
		b.Logger.Error("failed-to-list-deployments", err)
		return nil, errors.Wrap(err, "failed to list desired LRPs")
	}

	infos := toDesiredLRPSchedulingInfo(lrps)

	return infos, nil
}

func toDesiredLRPSchedulingInfo(lrps []opi.LRP) []*models.DesiredLRPSchedulingInfo {
	infos := []*models.DesiredLRPSchedulingInfo{}
	for _, l := range lrps {
		info := &models.DesiredLRPSchedulingInfo{}
		info.DesiredLRPKey.ProcessGuid = l.Metadata[cf.ProcessGuid]
		info.Annotation = l.Metadata[cf.LastUpdated]
		infos = append(infos, info)
	}
	return infos
}

func (b *Bifrost) Update(ctx context.Context, update models.UpdateDesiredLRPRequest) error {
	lrp, err := b.Desirer.Get(ctx, update.ProcessGuid)
	if err != nil {
		b.Logger.Error("application-not-found", err, lager.Data{"process-guid": update.ProcessGuid})
		return err
	}

	lrp.TargetInstances = int(*update.Update.Instances)
	lrp.Metadata[cf.LastUpdated] = *update.Update.Annotation

	return b.Desirer.Update(ctx, *lrp)
}

func (b *Bifrost) Get(ctx context.Context, guid string) *models.DesiredLRP {
	lrp, err := b.Desirer.Get(ctx, guid)
	if err != nil {
		b.Logger.Error("failed-to-get-deployment", err, lager.Data{"process-guid": guid})
		return nil
	}

	desiredLRP := &models.DesiredLRP{
		ProcessGuid: lrp.Name,
		Instances:   int32(lrp.TargetInstances),
	}

	return desiredLRP
}
