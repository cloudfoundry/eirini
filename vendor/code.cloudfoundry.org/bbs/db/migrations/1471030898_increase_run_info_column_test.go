package migrations_test

import (
	"strings"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/migration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Increase Run Info Column Migration", func() {
	var (
		migration migration.Migration
	)

	BeforeEach(func() {
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		migration = migrations.NewIncreaseRunInfoColumnSize()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1471030898))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			// Can't do this in the Describe BeforeEach
			// as the test on line 37 will cause ginkgo to panic
			migration.SetRawSQLDB(rawSQLDB)
			migration.SetDBFlavor(flavor)
		})

		BeforeEach(func() {
			createStatements := []string{
				`CREATE TABLE actual_lrps(
	net_info TEXT NOT NULL
);`,
				`CREATE TABLE tasks(
	result TEXT,
	task_definition TEXT NOT NULL
);`,

				`CREATE TABLE desired_lrps(
	annotation TEXT,
	routes TEXT NOT NULL,
	volume_placement TEXT NOT NULL,
	run_info TEXT NOT NULL
);`,
			}
			for _, st := range createStatements {
				_, err := rawSQLDB.Exec(st)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should change the size of all text columns ", func() {
			Expect(migration.Up(logger)).To(Succeed())
			value := strings.Repeat("x", 65536*2)
			query := helpers.RebindForFlavor("insert into desired_lrps(annotation, routes, volume_placement, run_info) values('', '', '', ?)", flavor)
			_, err := rawSQLDB.Exec(query, value)
			Expect(err).NotTo(HaveOccurred())
		})

		It("is idempotent", func() {
			testIdempotency(rawSQLDB, migration, logger)
		})
	})
})
