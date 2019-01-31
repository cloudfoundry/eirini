package migrations_test

import (
	"code.cloudfoundry.org/bbs/db/migrations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrations", func() {
	It("has no duplicate migrations", func() {
		allMigrations := make(map[int64]bool)
		for _, mig := range migrations.AllMigrations() {
			Expect(allMigrations).ToNot(HaveKey(mig.Version()), "Duplicate migration version found")
			allMigrations[mig.Version()] = true
		}
	})
})
