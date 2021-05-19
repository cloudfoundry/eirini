package stset

import (
	"context"

	"code.cloudfoundry.org/eirini/api"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
)

type StatefulsetGetter interface {
	GetByLRPIdentifier(ctx context.Context, id api.LRPIdentifier) ([]appsv1.StatefulSet, error)
}

type StatusGetter struct {
	logger         lager.Logger
	getStatefulSet getStatefulSetFunc
}

func NewStatusGetter(logger lager.Logger, statefulsetGetter StatefulsetGetter) StatusGetter {
	return StatusGetter{
		logger:         logger,
		getStatefulSet: newGetStatefulSetFunc(statefulsetGetter),
	}
}

func (g StatusGetter) GetStatus(ctx context.Context, identifier api.LRPIdentifier) (eiriniv1.LRPStatus, error) {
	statefulSet, err := g.getStatefulSet(ctx, identifier)
	if err != nil {
		return eiriniv1.LRPStatus{}, errors.Wrap(err, "failed to get statefulset for LRP")
	}

	return eiriniv1.LRPStatus{
		Replicas: statefulSet.Status.ReadyReplicas,
	}, nil
}
