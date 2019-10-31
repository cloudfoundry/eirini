package rootfspatcher

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RootfsVersionLabel = "cloudfoundry.org/rootfs-version"

//go:generate counterfeiter . StatefulSetUpdaterLister
type StatefulSetUpdaterLister interface {
	Update(*apps.StatefulSet) (*apps.StatefulSet, error)
	List(metav1.ListOptions) (*apps.StatefulSetList, error)
}

type StatefulSetPatcher struct {
	Version      string
	StatefulSets StatefulSetUpdaterLister
	Logger       lager.Logger
}

func (p StatefulSetPatcher) Patch() error {
	listOpts := metav1.ListOptions{}
	sts, err := p.StatefulSets.List(listOpts)
	if err != nil {
		return errors.Wrap(err, "failed to list statefulsets")
	}

	failuresOccured := 0
	p.Logger.Info(fmt.Sprintf("found %d stateful sets to patch", len(sts.Items)))
	for _, s := range sts.Items {
		statesfulset := s
		statesfulset.Labels[RootfsVersionLabel] = p.Version
		statesfulset.Spec.Template.Labels[RootfsVersionLabel] = p.Version
		_, err := p.StatefulSets.Update(&statesfulset)
		if err != nil {
			p.Logger.Error("failed to patch", err)
			failuresOccured++
		}
	}

	if failuresOccured > 0 {
		return errors.New(fmt.Sprintf("failed to update %d statefulsets", failuresOccured))
	}

	return nil
}
