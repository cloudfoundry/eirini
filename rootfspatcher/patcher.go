package rootfspatcher

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/apps/v1beta2"
)

const RootfsVersionLabel = "rootfs-version"

type StatefulSetPatcher struct {
	Version string
	Client  v1beta2.StatefulSetInterface
}

func (p StatefulSetPatcher) Patch() error {
	listOpts := metav1.ListOptions{}
	ss, err := p.Client.List(listOpts)
	if err != nil {
		return errors.Wrap(err, "failed to list statefulsets")
	}

	for _, s := range ss.Items {
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
