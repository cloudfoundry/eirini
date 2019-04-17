package rootfspatcher

import "github.com/pkg/errors"

//go:generate counterfeiter . Waiter
type Waiter interface {
	Wait() error
}

//go:generate counterfeiter . Patcher
type Patcher interface {
	Patch() error
}

func PatchAndWait(patcher Patcher, waiter Waiter) error {
	if err := patcher.Patch(); err != nil {
		return errors.Wrap(err, "failed to patch resources")
	}

	return errors.Wrap(waiter.Wait(), "failed to wait for update")
}
