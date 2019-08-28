package util

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

type Timeoutable = func(stop <-chan interface{})

func RunWithTimeout(f Timeoutable, d time.Duration) error {
	t := time.NewTimer(d)
	if d < 0 {
		return errors.New("provided timeout is not valid")
	}

	stop := make(chan interface{}, 1)
	defer close(stop)

	ready := make(chan interface{}, 1)
	go func() {
		f(stop)
		defer close(ready)
	}()

	select {
	case <-ready:
		return nil
	case <-t.C:
		return fmt.Errorf("timed out after %s", d.String())
	}
}
