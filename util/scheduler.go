package util

import (
	"fmt"
	"time"
)

type Task func() error

//go:generate counterfeiter . TaskScheduler
type TaskScheduler interface {
	Schedule(task Task)
}

type TickerTaskScheduler struct {
	Ticker *time.Ticker
}

func (t *TickerTaskScheduler) Schedule(task Task) {
	for range t.Ticker.C {
		if err := task(); err != nil {
			fmt.Println("Task failed to execute. Reason: ", err.Error())
		}
	}
}

type SimpleLoopScheduler struct{}

func (s *SimpleLoopScheduler) Schedule(task Task) {
	for {
		if err := task(); err != nil {
			fmt.Println("Task failed to execute. Reason: ", err.Error())
		}
	}
}
