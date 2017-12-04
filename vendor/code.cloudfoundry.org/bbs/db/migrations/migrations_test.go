package migrations_test

import (
	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/migration/migrationfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrations", func() {
	It("can append a migration to the list", func() {
		m := &migrationfakes.FakeMigration{}
		migrations.AppendMigration(m)
		Expect(migrations.Migrations).To(ContainElement(m))
	})

	It("prevents duplicate versions", func() {
		m1 := &migrationfakes.FakeMigration{}
		m1.VersionReturns(1234)
		migrations.AppendMigration(m1)
		Expect(migrations.Migrations).To(ContainElement(m1))

		Expect(func() {
			m2 := &migrationfakes.FakeMigration{}
			m2.VersionReturns(1234)
			migrations.AppendMigration(m2)
		}).To(Panic())
	})
})
