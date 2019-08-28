package util

import "time"

type PollFunc func() bool

func PollUntilTrue(f PollFunc, d time.Duration, stop <-chan interface{}) bool {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return false
		case <-ticker.C:
			if f() {
				return true
			}
		}
	}
}
