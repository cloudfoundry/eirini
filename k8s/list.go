package k8s

import (
	"context"

	"github.com/cloudfoundry-incubator/eirini/opi"
)

type Lister struct{}

func (l *Lister) List(ctx context.Context) (map[string]opi.LRP, error) {
	return nil, nil
}
