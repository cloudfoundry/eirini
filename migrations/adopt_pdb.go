package migrations

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

//counterfeiter:generate . PDBClient

type PDBClient interface {
	Get(ctx context.Context, namespace, name string) (*v1beta1.PodDisruptionBudget, error)
	SetOwner(ctx context.Context, pdb *v1beta1.PodDisruptionBudget, owner *appsv1.StatefulSet) (*v1beta1.PodDisruptionBudget, error)
}

type AdoptPDB struct {
	pdbClient PDBClient
}

func NewAdoptPDB(pdbClient PDBClient) AdoptPDB {
	return AdoptPDB{
		pdbClient: pdbClient,
	}
}

func (m AdoptPDB) Apply(ctx context.Context, obj runtime.Object) error {
	stSet, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return fmt.Errorf("expected *v1.StatefulSet, got: %T", obj)
	}

	if *stSet.Spec.Replicas <= 1 {
		return nil
	}

	pdb, err := m.pdbClient.Get(ctx, stSet.Namespace, stSet.Name)
	if err != nil {
		return errors.Wrap(err, "adopt-pdb-migration-get-pdb-failed")
	}

	if _, err := m.pdbClient.SetOwner(ctx, pdb, stSet); err != nil {
		return errors.Wrap(err, "adopt-pdb-migration-set-owner-failed")
	}

	return nil
}

func (m AdoptPDB) SequenceID() int {
	return AdoptPDBSequenceID
}
