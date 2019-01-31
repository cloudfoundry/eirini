package migrations_test

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/migration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IncreaseTaskErrorColumns", func() {
	var (
		migration migration.Migration
	)

	BeforeEach(func() {
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		migration = migrations.NewIncreaseTaskErrorColumns()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1528928964))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			migration.SetRawSQLDB(rawSQLDB)
			migration.SetDBFlavor(flavor)

			createStatement := `CREATE TABLE tasks(
	failure_reason VARCHAR(255) NOT NULL DEFAULT '',
	rejection_reason VARCHAR(255) NOT NULL DEFAULT ''
);`
			_, err := rawSQLDB.Exec(createStatement)
			Expect(err).NotTo(HaveOccurred())
		})

		testTableAndColumn := func(table, column string) {
			title := fmt.Sprintf("should change the size of %s column ", column)
			It(title, func() {
				Expect(migration.Up(logger)).To(Succeed())
				value := strings.Repeat("x", 1024)
				insertQuery := fmt.Sprintf("insert into %s(%s) values(?)", table, column)
				query := helpers.RebindForFlavor(insertQuery, flavor)
				_, err := rawSQLDB.Exec(query, value)
				Expect(err).NotTo(HaveOccurred())
				selectQuery := fmt.Sprintf("select %s from %s", column, table)
				row := rawSQLDB.QueryRow(selectQuery)
				Expect(err).NotTo(HaveOccurred())
				var actualValue string
				Expect(row.Scan(&actualValue)).To(Succeed())
				Expect(actualValue).To(Equal(value))
			})
		}

		testTableAndColumn("tasks", "failure_reason")
		testTableAndColumn("tasks", "rejection_reason")

		It("does not remove non null constraint", func() {
			Expect(migration.Up(logger)).To(Succeed())
			query := helpers.RebindForFlavor("insert into tasks(failure_reason) values(?)", flavor)
			_, err := rawSQLDB.Exec(query, nil)
			Expect(err).To(MatchError(ContainSubstring("null")))
		})

		It("is idempotent", func() {
			testIdempotency(rawSQLDB, migration, logger)
		})
	})
})
