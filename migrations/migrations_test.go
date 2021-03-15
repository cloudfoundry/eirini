package migrations_test

import (
	"fmt"
	"reflect"

	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Migration Provider", func() {
	var migrationProvider migrations.MigrationProvider

	newMigrationStep := func(step int) *migrationsfakes.FakeMigrationStep {
		s := new(migrationsfakes.FakeMigrationStep)
		s.SequenceIDReturns(step)

		return s
	}

	BeforeEach(func() {
		migrationProvider = migrations.NewMigrationStepsProvider([]migrations.MigrationStep{
			newMigrationStep(5),
			newMigrationStep(4),
			newMigrationStep(7),
			newMigrationStep(6),
		})
	})

	Describe("Provide", func() {
		It("returns a list of all migrations sorted by sequence id", func() {
			Expect(migrationProvider.Provide()).To(BeSorted())
		})
	})

	Describe("GetLatestMigrationIndex", func() {
		It("returns the biggest sequence id of all migration steps", func() {
			Expect(migrationProvider.GetLatestMigrationIndex()).To(Equal(7))
		})
	})
})

func BeSorted() types.GomegaMatcher {
	return &BeSortedMatcher{}
}

type BeSortedMatcher struct{}

func (matcher *BeSortedMatcher) Match(actual interface{}) (success bool, err error) {
	migrationSteps, ok := actual.([]migrations.MigrationStep)
	if !ok {
		return false, fmt.Errorf("Expected a value of type []migrations.MigrationStep got %q", reflect.TypeOf(actual))
	}

	maxSequenceID := -1

	for _, step := range migrationSteps {
		if step.SequenceID() <= maxSequenceID {
			return false, nil
		}

		maxSequenceID = step.SequenceID()
	}

	return true, nil
}

func (matcher *BeSortedMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected the migration steps to be sorted by sequence id, but was %v", sequenceIds(actual))
}

func (matcher *BeSortedMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected the migration steps to not be sorted by sequence id, but was %v", sequenceIds(actual))
}

func sequenceIds(actual interface{}) []int {
	ids := []int{}

	migrationSteps, _ := actual.([]migrations.MigrationStep)
	for _, step := range migrationSteps {
		ids = append(ids, step.SequenceID())
	}

	return ids
}
