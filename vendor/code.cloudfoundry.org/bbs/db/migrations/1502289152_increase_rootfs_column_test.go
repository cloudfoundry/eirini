package migrations_test

import (
	"strings"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/migration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Increase rootfs Column Migration", func() {
	var (
		migration migration.Migration
	)

	BeforeEach(func() {
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		migration = migrations.NewIncreaseRootFSColumnSize()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1502289152))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			migration.SetRawSQLDB(rawSQLDB)
			migration.SetDBFlavor(flavor)
		})

		BeforeEach(func() {
			createStatements := []string{
				`CREATE TABLE desired_lrps(
	rootfs VARCHAR(255) NOT NULL
);`,
			}
			for _, st := range createStatements {
				_, err := rawSQLDB.Exec(st)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("changes the size of the rootfs column", func() {
			Expect(migration.Up(logger)).To(Succeed())
			value := strings.Repeat("x", 1024)
			query := helpers.RebindForFlavor("insert into desired_lrps(rootfs) values(?)", flavor)
			_, err := rawSQLDB.Exec(query, value)
			Expect(err).NotTo(HaveOccurred())
		})

		It("is idempotent", func() {
			testIdempotency(rawSQLDB, migration, logger)
		})
	})
})
