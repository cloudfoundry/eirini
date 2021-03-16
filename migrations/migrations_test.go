package migrations_test

import (
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
