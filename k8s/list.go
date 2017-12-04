package k8s

import (
	"context"

	"github.com/julz/cube/opi"
)

type Lister struct{}

func (l *Lister) List(ctx context.Context) (map[string]opi.LRP, error) {
	return nil, nil
}
