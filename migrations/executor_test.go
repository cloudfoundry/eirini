package migrations_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Migration Executor", func() {
	var (
		stSetClient       *migrationsfakes.FakeStatefulsetsClient
		migrationProvider *migrationsfakes.FakeMigrationProvider
		executor          *migrations.Executor
		migrationError    error
		migrationStep4    *migrationsfakes.FakeMigrationStep
		migrationStep5    *migrationsfakes.FakeMigrationStep
		migrationStep6    *migrationsfakes.FakeMigrationStep
		migrationStep7    *migrationsfakes.FakeMigrationStep
		stSetv5           *appsv1.StatefulSet
		logger            *lagertest.TestLogger
	)

	expectNoMigrationOccurred := func() {
		Expect(migrationStep4.ApplyCallCount()).To(BeZero())
		Expect(migrationStep5.ApplyCallCount()).To(BeZero())
		Expect(migrationStep6.ApplyCallCount()).To(BeZero())
		Expect(migrationStep7.ApplyCallCount()).To(BeZero())
	}

	newStatefulSet := func(namespace, name, seq string) *appsv1.StatefulSet {
		s := new(appsv1.StatefulSet)
		s.Namespace = namespace
		s.Name = name
		s.Annotations = map[string]string{stset.AnnotationLatestMigration: seq}

		return s
	}

	newMigrationStep := func(step int) *migrationsfakes.FakeMigrationStep {
		s := new(migrationsfakes.FakeMigrationStep)
		s.SequenceIDReturns(step)

		return s
	}

	BeforeEach(func() {
		stSetv5 = newStatefulSet("ns1", "name1", "5")

		stSetClient = new(migrationsfakes.FakeStatefulsetsClient)
		stSetClient.GetBySourceTypeReturns([]appsv1.StatefulSet{*stSetv5}, nil)

		migrationStep4 = newMigrationStep(4)
		migrationStep5 = newMigrationStep(5)
		migrationStep6 = newMigrationStep(6)
		migrationStep7 = newMigrationStep(7)

		migrationProvider = new(migrationsfakes.FakeMigrationProvider)
		migrationProvider.ProvideReturns([]migrations.MigrationStep{migrationStep4, migrationStep5, migrationStep6, migrationStep7})

		logger = lagertest.NewTestLogger("migration-test")
		executor = migrations.NewExecutor(stSetClient, migrationProvider)
	})

	JustBeforeEach(func() {
		migrationError = executor.MigrateStatefulSets(logger)
	})

	It("can migrate stateful sets", func() {
		Expect(migrationError).NotTo(HaveOccurred())
	})

	It("lists LRP statefulsets", func() {
		Expect(stSetClient.GetBySourceTypeCallCount()).To(Equal(1))
		_, actualSourceType := stSetClient.GetBySourceTypeArgsForCall(0)
		Expect(actualSourceType).To(Equal(stset.AppSourceType))
	})

	It("applies migration steps 6 & 7 to stateful set on version 5", func() {
		Expect(migrationStep4.ApplyCallCount()).To(BeZero())
		Expect(migrationStep5.ApplyCallCount()).To(BeZero())
		Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
		Expect(migrationStep7.ApplyCallCount()).To(Equal(1))

		_, stSet := migrationStep6.ApplyArgsForCall(0)
		Expect(stSet).To(Equal(stSetv5))
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

	It("bumps the latest migration annotation on the stateful set", func() {
		Expect(stSetClient.SetAnnotationCallCount()).To(Equal(2))
		_, actualStSet, actualAnnotationName, actualAnnotationValue := stSetClient.SetAnnotationArgsForCall(0)
		Expect(actualStSet).To(Equal(stSetv5))
		Expect(actualAnnotationName).To(Equal(stset.AnnotationLatestMigration))
		Expect(actualAnnotationValue).To(Equal("6"))
	})

	When("bumping the migration annotation fails", func() {
		BeforeEach(func() {
			stSetClient.SetAnnotationReturns(nil, errors.New("nope"))
		})

		It("errors and stops migration", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("nope")))

			Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep7.ApplyCallCount()).To(BeZero())
		})
	})

	Describe("migration ordering", func() {
		var mingrationHistory []migrations.MigrationStep

		BeforeEach(func() {
			mingrationHistory = []migrations.MigrationStep{}

			addToMigrationHistory := func(step migrations.MigrationStep) func(ctx context.Context, o runtime.Object) error {
				return func(ctx context.Context, _ runtime.Object) error {
					mingrationHistory = append(mingrationHistory, step)

					return nil
				}
			}

			migrationStep6.ApplyStub = addToMigrationHistory(migrationStep6)
			migrationStep7.ApplyStub = addToMigrationHistory(migrationStep7)
		})

		It("applies the applicable steps in their sequence order", func() {
			Expect(mingrationHistory).To(BeSorted())
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

	When("migration application fails", func() {
		BeforeEach(func() {
			migrationStep6.ApplyReturns(errors.New("oops"))
		})

		It("returns the error and stops migration", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("oops")))
			Expect(migrationStep7.ApplyCallCount()).To(BeZero())
		})
	})

	When("a stateful set has a unparseable latest migration annotation", func() {
		BeforeEach(func() {
			stSetv5.Annotations[stset.AnnotationLatestMigration] = "nope"
		})

		It("returns the error and stops processing", func() {
			Expect(migrationError).To(MatchError(ContainSubstring("nope")))
			expectNoMigrationOccurred()
		})
	})

	When("a stateful set does not have the latest migration annotation set", func() {
		BeforeEach(func() {
			delete(stSetv5.Annotations, stset.AnnotationLatestMigration)
		})

		It("applies all the migrations", func() {
			Expect(migrationStep4.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep5.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep6.ApplyCallCount()).To(Equal(1))
			Expect(migrationStep7.ApplyCallCount()).To(Equal(1))
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
})
