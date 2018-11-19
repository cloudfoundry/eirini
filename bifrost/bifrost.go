package bifrost

import (
	"context"
	"encoding/json"

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

func (b *Bifrost) Update(ctx context.Context, update models.UpdateDesiredLRPRequest) error {
	lrp, err := b.Desirer.Get(update.ProcessGuid)
	if err != nil {
		b.Logger.Error("application-not-found", err, lager.Data{"process-guid": update.ProcessGuid})
		return err
	}

	lrp.TargetInstances = int(*update.Update.Instances)
	lrp.Metadata[cf.LastUpdated] = *update.Update.Annotation

	hostnames, err := getURIs(update)
	if err != nil {
		return err
	}

	hostnamesBytes, err := json.Marshal(hostnames)
	if err != nil {
		b.Logger.Error("failed-to-marshal hostnames", err, lager.Data{"process-guid": update.ProcessGuid})
		panic(err)
	}

	lrp.Metadata[cf.VcapAppUris] = string(hostnamesBytes)

	return b.Desirer.Update(lrp)
}

func (b *Bifrost) GetApp(ctx context.Context, guid string) *models.DesiredLRP {
	lrp, err := b.Desirer.Get(guid)
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

func (b *Bifrost) Stop(ctx context.Context, guid string) error {
	return b.Desirer.Stop(guid)
}

func (b *Bifrost) GetInstances(ctx context.Context, guid string) ([]*cf.Instance, error) {
	return b.Desirer.GetInstances(guid)
}

func getURIs(update models.UpdateDesiredLRPRequest) ([]string, error) {
	if !routesAvailable(update.Update.Routes) {
		return []string{}, nil
	}

	cfRouterRoutes := (*update.Update.Routes)["cf-router"]
	data, err := cfRouterRoutes.MarshalJSON()
	if err != nil {
		return []string{}, err
	}

	routes := []struct {
		Hostnames []string `json:"hostnames"`
		Port      int      `json:"port"`
	}{}
	err = json.Unmarshal(data, &routes)
	if err != nil {
		return []string{}, err
	}

	hostnames := []string{}
	for _, r := range routes {
		hostnames = append(hostnames, r.Hostnames...)
	}

	return hostnames, nil
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
