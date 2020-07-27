package util

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type Task func() error

//counterfeiter:generate . TaskScheduler

type TaskScheduler interface {
	Schedule(task Task)
}

type TickerTaskScheduler struct {
	Ticker *time.Ticker
	Logger lager.Logger
}

func (t *TickerTaskScheduler) Schedule(task Task) {
	for range t.Ticker.C {
		if err := task(); err != nil {
			t.Logger.Error("task-failed", err)
		}
	}
}

type SimpleLoopScheduler struct {
	CancelChan chan struct{}
	Logger     lager.Logger
}

func (s *SimpleLoopScheduler) Schedule(task Task) {
	for {
		select {
		case <-s.CancelChan:
			return
		default:
			if err := task(); err != nil {
				s.Logger.Error("task-failed", err)
			}
		}
	}
}
