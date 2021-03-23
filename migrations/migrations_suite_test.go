package migrations_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"code.cloudfoundry.org/eirini/migrations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func TestMigrations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migrations Suite")
}

var ctx context.Context

var _ = BeforeEach(func() {
	ctx = context.Background()
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
