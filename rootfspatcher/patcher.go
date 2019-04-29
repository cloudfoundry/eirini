package rootfspatcher

import (
	"fmt"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RootfsVersionLabel = "rootfs-version"

//go:generate counterfeiter . StatefulSetPatchLister
type StatefulSetPatchLister interface {
	Update(*apps.StatefulSet) (*apps.StatefulSet, error)
	List(metav1.ListOptions) (*apps.StatefulSetList, error)
}

type StatefulSetPatcher struct {
	Version string
	Client  StatefulSetPatchLister
}

func (p StatefulSetPatcher) Patch() error {
	listOpts := metav1.ListOptions{}
	ss, err := p.Client.List(listOpts)
	if err != nil {
		return errors.Wrap(err, "failed to list statefulsets")
	}

	for _, s := range ss.Items {
		fmt.Println("test!!!!!")
		statefulset := s
		statefulset.Labels[RootfsVersionLabel] = p.Version
		statefulset.Spec.Template.Labels[RootfsVersionLabel] = p.Version
		_, err := p.Client.Update(&statefulset)
		if err != nil {
			return errors.Wrap(err, "failed to update statefulset")
		}
	}
	return nil
}
