package route

import (
	"fmt"
	"time"
)

//go:generate counterfeiter . TaskScheduler
type TaskScheduler interface {
	Schedule(task func() error)
}

type TickerTaskScheduler struct {
	Ticker *time.Ticker
}

func (t *TickerTaskScheduler) Schedule(task func() error) {
	for range t.Ticker.C {
		if err := task(); err != nil {
			fmt.Println("Task failed to execute. Reason: ", err.Error())
		}
	}
}

type SimpleLoopScheduler struct{}

func (s *SimpleLoopScheduler) Schedule(task func() error) {
	for {
		if err := task(); err != nil {
			fmt.Println("Task failed to execute. Reason: ", err.Error())
		}
	}
}
