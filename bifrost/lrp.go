package bifrost

import (
	"context"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/models/cf"
	"github.com/pkg/errors"
)

//counterfeiter:generate . LRPConverter
//counterfeiter:generate . LRPClient
//counterfeiter:generate . LRPNamespacer

type LRPConverter interface {
	ConvertLRP(request cf.DesireLRPRequest) (api.LRP, error)
}

type LRPClient interface {
	Desire(ctx context.Context, namespace string, lrp *api.LRP, opts ...shared.Option) error
	List(ctx context.Context) ([]*api.LRP, error)
	Get(ctx context.Context, identifier api.LRPIdentifier) (*api.LRP, error)
	GetInstances(ctx context.Context, identifier api.LRPIdentifier) ([]*api.Instance, error)
	Update(ctx context.Context, lrp *api.LRP) error
	Stop(ctx context.Context, identifier api.LRPIdentifier) error
	StopInstance(ctx context.Context, identifier api.LRPIdentifier, index uint) error
}

type LRPNamespacer interface {
	GetNamespace(requestedNamespace string) string
}

type LRP struct {
	Converter  LRPConverter
	LRPClient  LRPClient
	Namespacer LRPNamespacer
}

func (l *LRP) Transfer(ctx context.Context, request cf.DesireLRPRequest) error {
	desiredLRP, err := l.Converter.ConvertLRP(request)
	if err != nil {
		return errors.Wrap(err, "failed to convert request")
	}

	namespace := l.Namespacer.GetNamespace(request.Namespace)

	return errors.Wrap(l.LRPClient.Desire(ctx, namespace, &desiredLRP), "failed to desire")
}

func (l *LRP) List(ctx context.Context) ([]cf.DesiredLRPSchedulingInfo, error) {
	lrps, err := l.LRPClient.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list desired LRPs")
	}

	return toDesiredLRPSchedulingInfo(lrps), nil
}

func toDesiredLRPSchedulingInfo(lrps []*api.LRP) []cf.DesiredLRPSchedulingInfo {
	infos := []cf.DesiredLRPSchedulingInfo{}

	for _, l := range lrps {
		info := cf.DesiredLRPSchedulingInfo{}
		info.DesiredLRPKey.ProcessGUID = l.LRPIdentifier.ProcessGUID()
		info.GUID = l.LRPIdentifier.GUID
		info.Version = l.LRPIdentifier.Version
		info.Annotation = l.LastUpdated
		infos = append(infos, info)
	}

	return infos
}

func (l *LRP) Update(ctx context.Context, request cf.UpdateDesiredLRPRequest) error {
	identifier := api.LRPIdentifier{
		GUID:    request.GUID,
		Version: request.Version,
	}

	lrp, err := l.LRPClient.Get(ctx, identifier)
	if err != nil {
		return errors.Wrap(err, "failed to get app")
	}

	lrp.TargetInstances = request.Update.Instances
	lrp.LastUpdated = request.Update.Annotation

	lrp.Image = request.Update.Image

	return errors.Wrap(l.LRPClient.Update(ctx, lrp), "failed to update")
}

func (l *LRP) GetApp(ctx context.Context, identifier api.LRPIdentifier) (cf.DesiredLRP, error) {
	lrp, err := l.LRPClient.Get(ctx, identifier)
	if err != nil {
		return cf.DesiredLRP{}, errors.Wrap(err, "failed to get app")
	}

	desiredLRP := cf.DesiredLRP{
		ProcessGUID: identifier.ProcessGUID(),
		Instances:   int32(lrp.TargetInstances),
		Annotation:  lrp.LastUpdated,
		Image:       lrp.Image,
	}

	return desiredLRP, nil
}

func (l *LRP) Stop(ctx context.Context, identifier api.LRPIdentifier) error {
	return errors.Wrap(l.LRPClient.Stop(ctx, identifier), "failed to stop app")
}

func (l *LRP) StopInstance(ctx context.Context, identifier api.LRPIdentifier, index uint) error {
	if err := l.LRPClient.StopInstance(ctx, identifier, index); err != nil {
		return errors.Wrap(err, "failed to stop instance")
	}

	return nil
}

func (l *LRP) GetInstances(ctx context.Context, identifier api.LRPIdentifier) ([]*cf.Instance, error) {
	instances, err := l.LRPClient.GetInstances(ctx, identifier)
	if err != nil {
		return []*cf.Instance{}, errors.Wrap(err, "failed to get instances for app")
	}

	cfInstances := make([]*cf.Instance, 0, len(instances))
	for _, i := range instances {
		cfInstances = append(cfInstances, &cf.Instance{
			Since:          i.Since,
			Index:          i.Index,
			State:          i.State,
			PlacementError: i.PlacementError,
		})
	}

	return cfInstances, nil
}
