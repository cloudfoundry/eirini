package k8s

import (
	"context"

	"code.cloudfoundry.org/eirini/opi"
)

type Lister struct{}

func (l *Lister) List(ctx context.Context) (map[string]opi.LRP, error) {
	return nil, nil
}
