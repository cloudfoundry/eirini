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
	TargetInstances  int
	RunningInstances int
	Metadata         map[string]string
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
	Desire(ctx context.Context, lrps []LRP) error
	List(ctx context.Context) ([]LRP, error)
	Get(ctx context.Context, name string) (*LRP, error)
	Update(ctx context.Context, updated LRP) error
	Stop(ctx context.Context, name string) error
}

type TaskDesirer interface {
	Desire(ctx context.Context, tasks []Task) error
}

type DesireTaskFunc func(ctx context.Context, tasks []Task) error

func (d DesireTaskFunc) Desire(ctx context.Context, tasks []Task) error {
	return d(ctx, tasks)
}
