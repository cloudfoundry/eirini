package stset

import (
	"context"

	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
)

//counterfeiter:generate . StatefulSetToLRPConverter
//counterfeiter:generate . StatefulSetsBySourceTypeGetter

type StatefulSetToLRPConverter interface {
	Convert(s appsv1.StatefulSet) (*opi.LRP, error)
}

type StatefulSetsBySourceTypeGetter interface {
	GetBySourceType(ctx context.Context, sourceType string) ([]appsv1.StatefulSet, error)
}

type Lister struct {
	logger                    lager.Logger
	statefulSetGetter         StatefulSetsBySourceTypeGetter
	statefulsetToLrpConverter StatefulSetToLRPConverter
}

func NewLister(
	logger lager.Logger,
	statefulSetGetter StatefulSetsBySourceTypeGetter,
	statefulsetToLrpConverter StatefulSetToLRPConverter,
) Lister {
	return Lister{
		logger:                    logger,
		statefulSetGetter:         statefulSetGetter,
		statefulsetToLrpConverter: statefulsetToLrpConverter,
	}
}

func (l *Lister) List(ctx context.Context) ([]*opi.LRP, error) {
	logger := l.logger.Session("list")

	statefulsets, err := l.statefulSetGetter.GetBySourceType(ctx, AppSourceType)
	if err != nil {
		logger.Error("failed-to-list-statefulsets", err)

		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	lrps, err := l.statefulSetsToLRPs(statefulsets)
	if err != nil {
		logger.Error("failed-to-map-statefulsets-to-lrps", err)

		return nil, errors.Wrap(err, "failed to map statefulsets to lrps")
	}

	return lrps, nil
}

func (l *Lister) statefulSetsToLRPs(statefulSets []appsv1.StatefulSet) ([]*opi.LRP, error) {
	lrps := []*opi.LRP{}

	for _, s := range statefulSets {
		lrp, err := l.statefulsetToLrpConverter.Convert(s)
		if err != nil {
			return nil, err
		}

		lrps = append(lrps, lrp)
	}

	return lrps, nil
}
