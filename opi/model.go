package opi

import (
	"context"
)

// An LRP, or long-running-process, is a stateless process
// where the scheduler should attempt to keep N copies running,
// killing and recreating as needed to maintain that guarantee
type LRP struct {
	Name            string
	Image           string
	Command         []string
	Env             map[string]string
	TargetInstances int
}

// A Task is a one-off process that is run exactly once and returns a
// result
type Task struct {
	Image   string
	Command []string
	Env     map[string]string
}

type Desirer interface {
	Desire(ctx context.Context, lrps []LRP) error
}

type DesireFunc func(ctx context.Context, lrp []LRP) error

func (d DesireFunc) Desire(ctx context.Context, lrp []LRP) error {
	return d(ctx, lrp)
}

type TaskDesirer interface {
	Desire(ctx context.Context, tasks []Task) error
}

type DesireTaskFunc func(ctx context.Context, tasks []Task) error

func (d DesireTaskFunc) Desire(ctx context.Context, tasks []Task) error {
	return d(ctx, tasks)
}
