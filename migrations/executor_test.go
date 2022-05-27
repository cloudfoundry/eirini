package migrations_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Migration Executor", func() {
	var (
		stSetClient       *migrationsfakes.FakeStatefulsetsClient
		jobClient         *migrationsfakes.FakeJobsClient
		migrationProvider *migrationsfakes.FakeMigrationProvider
		executor          *migrations.Executor
		migrationError    error
		migrationStep4    *migrationsfakes.FakeMigrationStep
		migrationStep5    *migrationsfakes.FakeMigrationStep
		migrationStep6    *migrationsfakes.FakeMigrationStep
		migrationStep7    *migrationsfakes.FakeMigrationStep
		migrationStep8    *migrationsfakes.FakeMigrationStep
		stSetv5           *appsv1.StatefulSet
		jobv5             *batchv1.Job
		logger            *tests.TestLogger
	)

	expectNoMigrationOccurred := func() {
		Expect(migrationStep4.ApplyCallCount()).To(BeZero())
		Expect(migrationStep5.ApplyCallCount()).To(BeZero())
		Expect(migrationStep6.ApplyCallCount()).To(BeZero())
		Expect(migrationStep7.ApplyCallCount()).To(BeZero())
		Expect(migrationStep8.ApplyCallCount()).To(BeZero())
	}

	newStatefulSet := func(namespace, name, seq string) *appsv1.StatefulSet {
		s := new(appsv1.StatefulSet)
		s.Namespace = namespace
		s.Name = name
		s.Annotations = map[string]string{shared.AnnotationLatestMigration: seq}

		return s
	}

	newJob := func(namespace, name, seq string) *batchv1.Job {
		j := new(batchv1.Job)
		j.Namespace = namespace
		j.Name = name
		j.Annotations = map[string]string{shared.AnnotationLatestMigration: seq}

		return j
	}

	newMigrationStep := func(step int, desiredType migrations.ObjectType) *migrationsfakes.FakeMigrationStep {
		s := new(migrationsfakes.FakeMigrationStep)
		s.SequenceIDReturns(step)
		s.AppliesToReturns(desiredType)

		return s
	}

	BeforeEach(func() {
		stSetv5 = newStatefulSet("ns1", "name1", "5")
		jobv5 = newJob("ns1", "jobname", "5")

		stSetClient = new(migrationsfakes.FakeStatefulsetsClient)
		stSetClient.GetBySourceTypeReturns([]appsv1.StatefulSet{*stSetv5}, nil)

		jobClient = new(migrationsfakes.FakeJobsClient)
		jobClient.ListReturns([]batchv1.Job{*jobv5}, nil)

		migrationStep4 = newMigrationStep(4, migrations.StatefulSetObjectType)
		migrationStep5 = newMigrationStep(5, migrations.StatefulSetObjectType)
		migrationStep6 = newMigrationStep(6, migrations.StatefulSetObjectType)
		migrationStep7 = newMigrationStep(7, migrations.StatefulSetObjectType)
		migrationStep8 = newMigrationStep(8, migrations.JobObjectType)

		migrationProvider = new(migrationsfakes.FakeMigrationProvider)
		migrationProvider.ProvideReturns([]migrations.MigrationStep{migrationStep4, migrationStep5, migrationStep6, migrationStep7, migrationStep8})

		logger = tests.NewTestLogger("migration-test")
		executor = migrations.NewExecutor(stSetClient, jobClient, migrationProvider)
	})

	JustBeforeEach(func() {
		migrationError = executor.Migrate(context.Background(), logger)
	})

	It("can migrate stateful sets and jobs", func() {
		Expect(migrationError).NotTo(HaveOccurred())
	})

	It("lists statefulsets and jobs", func() {
		Expect(stSetClient.GetBySourceTypeCallCount()).To(Equal(1))
		_, actualSourceType := stSetClient.GetBySourceTypeArgsForCall(0)
		Expect(actualSourceType).To(Equal(stset.AppSourceType))

		Expect(jobClient.ListCallCount()).To(Equal(1))
		_, actualIncludeComplete := jobClient.ListArgsForCall(0)
		Expect(actualIncludeComplete).To(Equal(true))
	})

	It("checks the migration object type", func() {
		Expect(migrationStep7.AppliesToCallCount()).To(Equal(2))
		Expect(migrationStep8.AppliesToCallCount()).To(Equal(2))
	})

	It("applies migration steps 6, 7 and 8", func() {
		Expect(migrationStep4.ApplyCallCount()).To(BeZero())
		Expect(migrationStep5.ApplyCallCount()).To(BeZero())
		Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
		Expect(migrationStep7.ApplyCallCount()).To(Equal(1))
		Expect(migrationStep8.ApplyCallCount()).To(Equal(1))

		_, stSet := migrationStep6.ApplyArgsForCall(0)
		Expect(stSet).To(Equal(stSetv5))

		_, job := migrationStep8.ApplyArgsForCall(0)
		Expect(job).To(Equal(jobv5))
	})

	When("there is more than one stateful set listed", func() {
		BeforeEach(func() {
			stSetv6 := newStatefulSet("ns2", "name2", "6")
			stSetClient.GetBySourceTypeReturns([]appsv1.StatefulSet{*stSetv5, *stSetv6}, nil)
		})

		It("applies the migration steps with sequence > st set migtation annotation", func() {
			Expect(migrationStep4.ApplyCallCount()).To(BeZero())
			Expect(migrationStep5.ApplyCallCount()).To(BeZero())
			Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep7.ApplyCallCount()).To(Equal(2))
		})
	})

	It("bumps the latest migration annotations", func() {
		Expect(stSetClient.SetAnnotationCallCount()).To(Equal(2))
		_, actualStSet, actualAnnotationName, actualAnnotationValue := stSetClient.SetAnnotationArgsForCall(0)
		Expect(actualStSet).To(Equal(stSetv5))
		Expect(actualAnnotationName).To(Equal(shared.AnnotationLatestMigration))
		Expect(actualAnnotationValue).To(Equal("6"))

		Expect(jobClient.SetAnnotationCallCount()).To(Equal(1))
		_, job, name, val := jobClient.SetAnnotationArgsForCall(0)
		Expect(job).To(Equal(jobv5))
		Expect(name).To(Equal(shared.AnnotationLatestMigration))
		Expect(val).To(Equal("8"))
	})

	When("bumping the stateful set migration annotation fails", func() {
		BeforeEach(func() {
			stSetClient.SetAnnotationReturns(nil, errors.New("nope"))
		})

		It("errors and stops migration", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("nope")))

			Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep7.ApplyCallCount()).To(BeZero())
		})
	})

	When("bumping the job migration annotation fails", func() {
		BeforeEach(func() {
			jobClient.SetAnnotationReturns(nil, errors.New("nope"))
		})

		It("errors", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("nope")))
		})
	})

	Describe("migration ordering", func() {
		var migrationHistory []migrations.MigrationStep

		BeforeEach(func() {
			migrationHistory = []migrations.MigrationStep{}

			addToMigrationHistory := func(step migrations.MigrationStep) func(ctx context.Context, o runtime.Object) error {
				return func(ctx context.Context, _ runtime.Object) error {
					migrationHistory = append(migrationHistory, step)

					return nil
				}
			}

			migrationStep6.ApplyStub = addToMigrationHistory(migrationStep6)
			migrationStep7.ApplyStub = addToMigrationHistory(migrationStep7)
		})

		It("applies the applicable steps in their sequence order", func() {
			Expect(migrationHistory).To(BeSorted())
		})
	})

	Describe("sequence ID validation", func() {
		When("sequence IDs are not unique", func() {
			BeforeEach(func() {
				migrationStep7.SequenceIDReturns(6)
			})

			It("fais the migration and does not apply migration steps", func() {
				Expect(migrationError).To(MatchError(ContainSubstring("duplicate")))
				expectNoMigrationOccurred()
			})
		})

		When("a migration sequence ID is negative", func() {
			BeforeEach(func() {
				migrationStep7.SequenceIDReturns(-6)
			})

			It("fais the migration and does not apply migration steps", func() {
				Expect(migrationError).To(MatchError(ContainSubstring("negative")))
				expectNoMigrationOccurred()
			})
		})
	})

	When("stateful set migration application fails", func() {
		BeforeEach(func() {
			migrationStep6.ApplyReturns(errors.New("oops"))
		})

		It("returns the error and stops migration", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("oops")))
			Expect(migrationStep7.ApplyCallCount()).To(BeZero())
			Expect(migrationStep8.ApplyCallCount()).To(BeZero())
		})
	})

	When("job migration application fails", func() {
		BeforeEach(func() {
			migrationStep8.ApplyReturns(errors.New("oops"))
		})

		It("returns the error", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("oops")))
		})
	})

	When("a stateful set has a unparseable latest migration annotation", func() {
		BeforeEach(func() {
			stSetv5.Annotations[shared.AnnotationLatestMigration] = "nope"
		})

		It("returns the error and stops processing", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("nope")))
			expectNoMigrationOccurred()
		})
	})

	When("a job has a unparseable latest migration annotation", func() {
		BeforeEach(func() {
			jobv5.Annotations[shared.AnnotationLatestMigration] = "nope"
		})

		It("returns the error", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("nope")))
		})
	})

	When("an object does not have the latest migration annotation set", func() {
		BeforeEach(func() {
			delete(stSetv5.Annotations, shared.AnnotationLatestMigration)
			delete(jobv5.Annotations, shared.AnnotationLatestMigration)
		})

		It("applies all the migrations", func() {
			Expect(migrationStep4.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep5.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep7.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep8.ApplyCallCount()).To(Equal(1))
		})
	})

	When("listing stateful sets fails", func() {
		BeforeEach(func() {
			stSetClient.GetBySourceTypeReturns(nil, errors.New("boom"))
		})

		It("fails the migration", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("boom")))
		})

		It("carries out no migrations", func() {
			expectNoMigrationOccurred()
		})
	})

	When("listing jobs fails", func() {
		BeforeEach(func() {
			jobClient.ListReturns(nil, errors.New("boom"))
		})

		It("fails the migration", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("boom")))
		})

		It("carries out no job migrations", func() {
			Expect(migrationStep8.ApplyCallCount()).To(Equal(0))
		})
	})
})
