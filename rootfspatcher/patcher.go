package rootfspatcher

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/apps/v1beta2"
)

const RootfsVersionLabel = "rootfs-version"

//go:generate counterfeiter . Waiter
type Waiter interface {
	Wait() error
}

type Patcher struct {
	Version string
	Client  v1beta2.StatefulSetInterface
	Waiter  Waiter
}

func (p Patcher) Patch() error {
	listOpts := metav1.ListOptions{}
	ss, err := p.Client.List(listOpts)
	if err != nil {
		return errors.Wrap(err, "failed to list statefulsets")
	}

	for _, statefulset := range ss.Items {
		statefulset.Labels[RootfsVersionLabel] = p.Version
		_, err := p.Client.Update(&statefulset)
		if err != nil {
			return errors.Wrap(err, "failed to update statefulset")
		}

	}
	err = p.Waiter.Wait()
	return errors.Wrap(err, "failed to wait for update")
}
