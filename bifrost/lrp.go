package bifrost

import (
	"context"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
)

//counterfeiter:generate . LRPConverter
type LRPConverter interface {
	ConvertLRP(request cf.DesireLRPRequest) (opi.LRP, error)
}

//counterfeiter:generate . LRPDesirer
type LRPDesirer interface {
	Desire(lrp *opi.LRP) error
	List() ([]*opi.LRP, error)
	Get(identifier opi.LRPIdentifier) (*opi.LRP, error)
	GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error)
	Update(lrp *opi.LRP) error
	Stop(identifier opi.LRPIdentifier) error
	StopInstance(identifier opi.LRPIdentifier, index uint) error
}

type LRP struct {
	Converter LRPConverter
	Desirer   LRPDesirer
}

func (l *LRP) Transfer(ctx context.Context, request cf.DesireLRPRequest) error {
	desiredLRP, err := l.Converter.ConvertLRP(request)
	if err != nil {
		return errors.Wrap(err, "failed to convert request")
	}
	return errors.Wrap(l.Desirer.Desire(&desiredLRP), "failed to desire")
}

func (l *LRP) List(ctx context.Context) ([]*models.DesiredLRPSchedulingInfo, error) {
	lrps, err := l.Desirer.List()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list desired LRPs")
	}

	return toDesiredLRPSchedulingInfo(lrps), nil
}

func toDesiredLRPSchedulingInfo(lrps []*opi.LRP) []*models.DesiredLRPSchedulingInfo {
	infos := []*models.DesiredLRPSchedulingInfo{}
	for _, l := range lrps {
		info := &models.DesiredLRPSchedulingInfo{}
		info.DesiredLRPKey.ProcessGuid = l.LRPIdentifier.ProcessGUID()
		info.Annotation = l.LastUpdated
		infos = append(infos, info)
	}
	return infos
}

func (l *LRP) Update(ctx context.Context, update cf.UpdateDesiredLRPRequest) error {
	identifier := opi.LRPIdentifier{
		GUID:    update.GUID,
		Version: update.Version,
	}

	lrp, err := l.Desirer.Get(identifier)
	if err != nil {
		return errors.Wrap(err, "failed to get app")
	}

	u := update.GetUpdate()

	lrp.TargetInstances = int(u.GetInstances())
	lrp.LastUpdated = u.GetAnnotation()

	lrp.AppURIs = getURIs(update)
	return errors.Wrap(l.Desirer.Update(lrp), "failed to update")
}

func (l *LRP) GetApp(ctx context.Context, identifier opi.LRPIdentifier) (*models.DesiredLRP, error) {
	lrp, err := l.Desirer.Get(identifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get app")
	}

	desiredLRP := &models.DesiredLRP{
		ProcessGuid: identifier.ProcessGUID(),
		Instances:   int32(lrp.TargetInstances),
	}

	return desiredLRP, nil
}

func (l *LRP) Stop(ctx context.Context, identifier opi.LRPIdentifier) error {
	return errors.Wrap(l.Desirer.Stop(identifier), "failed to stop app")
}

func (l *LRP) StopInstance(ctx context.Context, identifier opi.LRPIdentifier, index uint) error {
	if err := l.Desirer.StopInstance(identifier, index); err != nil {
		return errors.Wrap(err, "failed to stop instance")
	}
	return nil
}

func (l *LRP) GetInstances(ctx context.Context, identifier opi.LRPIdentifier) ([]*cf.Instance, error) {
	opiInstances, err := l.Desirer.GetInstances(identifier)
	if err != nil {
		return []*cf.Instance{}, errors.Wrap(err, "failed to get instances for app")
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

func getURIs(update cf.UpdateDesiredLRPRequest) string {
	if !routesAvailable(update.Update.Routes) {
		return ""
	}

	cfRouterRoutes := (*update.Update.Routes)["cf-router"]
	data, err := cfRouterRoutes.MarshalJSON()
	if err != nil {
		panic("This should never happen")
	}

	return string(data)
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
