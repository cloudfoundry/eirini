package opi

import (
	"context"
)

// An LRP, or long-running-process, is a stateless process
// where the scheduler should attempt to keep N copies running,
// killing and recreating as needed to maintain that guarantee
type LRP struct {
	Name             string
	Image            string
	Command          []string
	Env              map[string]string
	Health           Healtcheck
	TargetInstances  int
	RunningInstances int
	Metadata         map[string]string
}

type Healtcheck struct {
	Type      string
	Port      int32
	Endpoint  string
	TimeoutMs uint
}

// A Task is a one-off process that is run exactly once and returns a
// result
type Task struct {
	Image   string
	Command []string
	Env     map[string]string
}

//go:generate counterfeiter . Desirer
type Desirer interface {
	Desire(lrp *LRP) error
	List() ([]*LRP, error)
	Get(name string) (*LRP, error)
	Update(lrp *LRP) error
	Stop(name string) error
}

type TaskDesirer interface {
	Desire(ctx context.Context, tasks []Task) error
}

type DesireTaskFunc func(ctx context.Context, tasks []Task) error

func (d DesireTaskFunc) Desire(ctx context.Context, tasks []Task) error {
	return d(ctx, tasks)
}
