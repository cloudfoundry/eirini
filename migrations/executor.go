// Package migrations organises required migrations of eirini managed k8s
// objects
package migrations

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/lager"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//counterfeiter:generate . StatefulsetsClient

type StatefulsetsClient interface {
	GetBySourceType(ctx context.Context, sourceType string) ([]appsv1.StatefulSet, error)
	SetAnnotation(ctx context.Context, statefulSet *appsv1.StatefulSet, key, value string) (*appsv1.StatefulSet, error)
}

//counterfeiter:generate . MigrationStep

type MigrationStep interface {
	Apply(ctx context.Context, obj runtime.Object) error
	SequenceID() int
}

//counterfeiter:generate . MigrationProvider

type MigrationProvider interface {
	Provide() []MigrationStep
	GetLatestMigrationIndex() int
}

type Executor struct {
	stSetClient    StatefulsetsClient
	migrationSteps []MigrationStep
}

func NewExecutor(stSetClient StatefulsetsClient, migrationStepProvider MigrationProvider) *Executor {
	return &Executor{
		stSetClient:    stSetClient,
		migrationSteps: migrationStepProvider.Provide(),
	}
}

func (e *Executor) MigrateStatefulSets(logger lager.Logger) error {
	logger.Info("migration-start")
	defer logger.Info("migration-end")

	ctx := context.Background()

	if err := e.verifySequenceIDs(); err != nil {
		logger.Error("migration-sequence-ids-error", err)

		return fmt.Errorf("problem with sequence IDs: %w", err)
	}

	stSets, err := e.stSetClient.GetBySourceType(ctx, stset.AppSourceType)
	if err != nil {
		logger.Error("get-stateful-sets-error", err)

		return fmt.Errorf("getting stateful sets failed: %w", err)
	}

	for i := range stSets {
		stSet := stSets[i]
		logger = logger.WithData(lager.Data{"namespace": stSet.Namespace, "name": stSet.Name})

		migrationAnnotationValue, err := parseLatestMigration(stSet.Annotations[stset.AnnotationLatestMigration])
		if err != nil {
			logger.Error("cannot-parse-latest-migration-annotation", err)

			return fmt.Errorf("failed to parse latest migration annotation for statefulset: %w", err)
		}

		for _, step := range e.migrationSteps {
			seq := step.SequenceID()
			if migrationAnnotationValue >= seq {
				continue
			}

			logger = logger.WithData(lager.Data{"sequence-id": seq})
			logger.Debug("applying-migration")

			if err := step.Apply(ctx, &stSet); err != nil {
				logger.Error("migration-failed", err)

				return fmt.Errorf("failed to apply migration: %w", err)
			}

			if _, err := e.stSetClient.SetAnnotation(ctx, &stSet, stset.AnnotationLatestMigration, strconv.Itoa(seq)); err != nil {
				logger.Error("patch-migration-annotation-failed", err)

				return fmt.Errorf("failed patching stateful set to set migration annotation: %w", err)
			}
		}
	}

	return nil
}

func (e *Executor) verifySequenceIDs() error {
	ids := make(map[int]int, len(e.migrationSteps))

	for _, m := range e.migrationSteps {
		id := m.SequenceID()
		ids[id]++

		if ids[id] > 1 {
			return fmt.Errorf("duplicate SequenceID %d", id)
		}

		if id < 0 {
			return fmt.Errorf("negative SequenceID %d", id)
		}
	}

	return nil
}

func parseLatestMigration(annotationValue string) (int, error) {
	if annotationValue == "" {
		return 0, nil
	}

	return strconv.Atoi(annotationValue)
}
