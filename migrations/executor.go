// Package migrations organises required migrations of eirini managed k8s
// objects
package migrations

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/lager"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//counterfeiter:generate . StatefulsetsClient

type StatefulsetsClient interface {
	GetBySourceType(ctx context.Context, sourceType string) ([]appsv1.Deployment, error)
	SetAnnotation(ctx context.Context, statefulSet *appsv1.Deployment, key, value string) (*appsv1.Deployment, error)
}

//counterfeiter:generate . JobsClient

type JobsClient interface {
	SetAnnotation(ctx context.Context, job *batchv1.Job, key, value string) (*batchv1.Job, error)
	List(ctx context.Context, includeCompleted bool) ([]batchv1.Job, error)
}

type AnnotationSetter interface {
	SetAnnotation(ctx context.Context, obj runtime.Object, key, value string) (runtime.Object, error)
}

//counterfeiter:generate . MigrationStep

type MigrationStep interface {
	Apply(ctx context.Context, obj runtime.Object) error
	SequenceID() int
	AppliesTo() ObjectType
}

//counterfeiter:generate . MigrationProvider

type MigrationProvider interface {
	Provide() []MigrationStep
	GetLatestMigrationIndex() int
}

type Executor struct {
	stSetClient    StatefulsetsClient
	jobsClient     JobsClient
	migrationSteps []MigrationStep
}

func NewExecutor(stSetClient StatefulsetsClient, jobsClient JobsClient, migrationStepProvider MigrationProvider) *Executor {
	return &Executor{
		stSetClient:    stSetClient,
		jobsClient:     jobsClient,
		migrationSteps: migrationStepProvider.Provide(),
	}
}

func (e *Executor) Migrate(ctx context.Context, logger lager.Logger) error {
	logger.Info("migration-start")
	defer logger.Info("migration-end")

	if err := e.verifySequenceIDs(); err != nil {
		logger.Error("migration-sequence-ids-error", err)

		return fmt.Errorf("problem with sequence IDs: %w", err)
	}

	stSetObjs, err := e.getStatefulSetsObjects(ctx)
	if err != nil {
		return err
	}

	err = e.migrateObjects(
		ctx,
		logger.Session("migrate-stateful-sets"),
		stSetObjs,
		StatefulSetObjectType,
		e.setStatefulSetAnnotation,
	)
	if err != nil {
		return err
	}

	jobObjs, err := e.getJobObjects(ctx)
	if err != nil {
		return err
	}

	return e.migrateObjects(
		ctx,
		logger.Session("migrate-jobs"),
		jobObjs,
		JobObjectType,
		e.setJobAnnotation,
	)
}

func (e *Executor) migrateObjects(ctx context.Context, logger lager.Logger, objects []runtime.Object, objectType ObjectType, setAnnotationFn func(context.Context, runtime.Object, int) error) error {
	logger.Info("start")
	defer logger.Info("end")

	for i := range objects {
		metaObject, ok := objects[i].(metav1.Object)
		if !ok {
			return fmt.Errorf("expected metav1.Object, got %T", objects[i])
		}

		logger = logger.WithData(lager.Data{"namespace": metaObject.GetNamespace(), "name": metaObject.GetName()})

		migrationAnnotationValue, err := parseLatestMigration(metaObject.GetAnnotations()[shared.AnnotationLatestMigration])
		if err != nil {
			logger.Error("cannot-parse-latest-migration-annotation", err)

			return fmt.Errorf("failed to parse latest migration annotation for statefulset: %w", err)
		}

		logger = logger.WithData(lager.Data{"last-migration": migrationAnnotationValue})
		logger.Debug("applying-steps")

		for _, step := range e.migrationSteps {
			seq := step.SequenceID()
			logger = logger.WithData(lager.Data{"step-id": seq})

			if migrationAnnotationValue >= seq {
				logger.Debug("skipping-applied-step")

				continue
			}

			if step.AppliesTo() != objectType {
				logger.Debug("skipping-other-object-type")

				continue
			}

			logger.Debug("applying-migration")

			if err := step.Apply(ctx, objects[i]); err != nil {
				logger.Error("migration-failed", err)

				return fmt.Errorf("failed to apply migration: %w", err)
			}

			if err := setAnnotationFn(ctx, objects[i], seq); err != nil {
				logger.Error("patch-migration-annotation-failed", err)

				return fmt.Errorf("failed patching stateful set to set migration annotation: %w", err)
			}
		}
	}

	return nil
}

func (e *Executor) setStatefulSetAnnotation(ctx context.Context, obj runtime.Object, seq int) error {
	stSet, ok := obj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("expected *appsv1.Deployment, got %T", obj)
	}

	if _, err := e.stSetClient.SetAnnotation(ctx, stSet, shared.AnnotationLatestMigration, strconv.Itoa(seq)); err != nil {
		return fmt.Errorf("failed patching stateful set to set migration annotation: %w", err)
	}

	return nil
}

func (e *Executor) setJobAnnotation(ctx context.Context, obj runtime.Object, seq int) error {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return fmt.Errorf("expected *batchv1.Job got %T", obj)
	}

	if _, err := e.jobsClient.SetAnnotation(ctx, job, shared.AnnotationLatestMigration, strconv.Itoa(seq)); err != nil {
		return fmt.Errorf("failed patching stateful set to set migration annotation: %w", err)
	}

	return nil
}

func (e *Executor) getStatefulSetsObjects(ctx context.Context) ([]runtime.Object, error) {
	stSets, err := e.stSetClient.GetBySourceType(ctx, stset.AppSourceType)
	if err != nil {
		return nil, fmt.Errorf("getting stateful sets failed: %w", err)
	}

	stSetObjs := []runtime.Object{}
	for i := range stSets {
		stSetObjs = append(stSetObjs, &stSets[i])
	}

	return stSetObjs, nil
}

func (e *Executor) getJobObjects(ctx context.Context) ([]runtime.Object, error) {
	jobs, err := e.jobsClient.List(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("getting jobs failed: %w", err)
	}

	jobObjs := []runtime.Object{}
	for i := range jobs {
		jobObjs = append(jobObjs, &jobs[i])
	}

	return jobObjs, nil
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
