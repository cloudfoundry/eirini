package st8ger

import (
	"context"

	"github.com/julz/cube/opi"
)

type St8ger struct {
	Desirer opi.TaskDesirer
}

func (s St8ger) Run(task opi.Task) error {
	err := s.Desirer.Desire(context.Background(), []opi.Task{task})
	if err != nil {
		return err
	}
	return nil
}
