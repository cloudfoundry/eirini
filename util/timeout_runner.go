package util

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Timeoutable = func(ready chan<- interface{}, stop <-chan interface{})

func RunWithTimeout(f Timeoutable, d time.Duration) error {
	ready := make(chan interface{}, 1)
	defer close(ready)

	stop := make(chan interface{}, 1)
	defer close(stop)

	t := time.NewTimer(d)
	if d < 0 {
		return errors.New("provided timeout is not valid")
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		f(ready, stop)
		wg.Done()
	}()

	functionExited := make(chan interface{}, 1)

	go func() {
		wg.Wait()
		functionExited <- nil
	}()

	select {
	case <-ready:
		return nil
	case <-t.C:
		return fmt.Errorf("timed out after %s", d.String())
	case <-functionExited:
		return errors.New("function completed before timeout, but did not write to ready chan")
	}
}
