package bifrost

import (
	"context"
	"fmt"

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

func (b *Bifrost) Transfer(ctx context.Context, request cf.DesireLRPRequest) error {
	desiredLRP, err := b.Converter.Convert(request)
	if err != nil {
		b.Logger.Error("failed-to-convert-request", err, lager.Data{"desire-lrp-request": request})
		return err
	}
	return b.Desirer.Desire(&desiredLRP)
}

func (b *Bifrost) List(ctx context.Context) ([]*models.DesiredLRPSchedulingInfo, error) {
	lrps, err := b.Desirer.List()
	if err != nil {
		b.Logger.Error("failed-to-list-deployments", err)
		return nil, errors.Wrap(err, "failed to list desired LRPs")
	}

	infos := toDesiredLRPSchedulingInfo(lrps)

	return infos, nil
}

func toDesiredLRPSchedulingInfo(lrps []*opi.LRP) []*models.DesiredLRPSchedulingInfo {
	infos := []*models.DesiredLRPSchedulingInfo{}
	for _, l := range lrps {
		info := &models.DesiredLRPSchedulingInfo{}
		info.DesiredLRPKey.ProcessGuid = l.Metadata[cf.ProcessGUID]
		info.Annotation = l.Metadata[cf.LastUpdated]
		infos = append(infos, info)
	}
	return infos
}

func (b *Bifrost) Update(ctx context.Context, update cf.UpdateDesiredLRPRequest) error {
	identifier := opi.LRPIdentifier{
		GUID:    update.GUID,
		Version: update.Version,
	}

	lrp, err := b.Desirer.Get(identifier)
	if err != nil {
		b.Logger.Error("application-not-found", err, lager.Data{"process-guid": update.ProcessGuid})
		return err
	}

	lrp.TargetInstances = int(*update.Update.Instances)
	lrp.Metadata[cf.LastUpdated] = *update.Update.Annotation

	routes, err := getURIs(update)
	if err != nil {
		return err
	}

	lrp.Metadata[cf.VcapAppUris] = routes

	return b.Desirer.Update(lrp)
}

func (b *Bifrost) GetApp(ctx context.Context, identifier opi.LRPIdentifier) *models.DesiredLRP {
	lrp, err := b.Desirer.Get(identifier)
	if err != nil {
		b.Logger.Error("failed-to-get-deployment", err, lager.Data{"process-guid": identifier.GUID})
		return nil
	}

	desiredLRP := &models.DesiredLRP{
		ProcessGuid: identifier.ProcessGUID(),
		Instances:   int32(lrp.TargetInstances),
	}

	return desiredLRP
}

func (b *Bifrost) Stop(ctx context.Context, identifier opi.LRPIdentifier) error {
	return b.Desirer.Stop(identifier)
}

func (b *Bifrost) StopInstance(ctx context.Context, identifier opi.LRPIdentifier, index uint) error {
	err := b.Desirer.StopInstance(identifier, index)
	return errors.Wrap(err, "desirer failed to stop instance")
}

func (b *Bifrost) GetInstances(ctx context.Context, identifier opi.LRPIdentifier) ([]*cf.Instance, error) {
	opiInstances, err := b.Desirer.GetInstances(identifier)
	if err != nil {
		return []*cf.Instance{}, errors.Wrap(err, fmt.Sprintf("failed to get instances for app with guid: %s", identifier.GUID))
	}

	cfInstances := make([]*cf.Instance, 0, len(opiInstances))
	for _, i := range opiInstances {
		cfInstances = append(cfInstances, &cf.Instance{
			Since:          i.Since,
			Index:          i.Index,
			State:          i.State,
			PlacementError: i.PlacementError,
		})
	}

	return cfInstances, nil
}

func getURIs(update cf.UpdateDesiredLRPRequest) (string, error) {
	if !routesAvailable(update.Update.Routes) {
		return "", nil
	}

	cfRouterRoutes := (*update.Update.Routes)["cf-router"]
	data, err := cfRouterRoutes.MarshalJSON()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func routesAvailable(routes *models.Routes) bool {
	if routes == nil {
		return false
	}

	if _, ok := (*routes)["cf-router"]; !ok {
		return false
	}

	return true
}
