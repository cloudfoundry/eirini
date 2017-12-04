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

var _ = Describe("Increase Error Columns Migration", func() {
	var (
		migration    migration.Migration
		migrationErr error
	)

	BeforeEach(func() {
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		migration = migrations.NewIncreaseErrorColumnsSize()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.Migrations).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1474908092))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			// Can't do this in the Describe BeforeEach
			// as the test on line 37 will cause ginkgo to panic
			migration.SetRawSQLDB(rawSQLDB)
			migration.SetDBFlavor(flavor)
		})

		JustBeforeEach(func() {
			migrationErr = migration.Up(logger)
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			createStatement := `CREATE TABLE actual_lrps(
	placement_error VARCHAR(255) NOT NULL DEFAULT '',
	crash_reason VARCHAR(255) NOT NULL DEFAULT ''
);`
			_, err := rawSQLDB.Exec(createStatement)
			Expect(err).NotTo(HaveOccurred())
		})

		testTableAndColumn := func(table, column string) {
			title := fmt.Sprintf("should change the size of %s column ", column)
			It(title, func() {
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

		testTableAndColumn("actual_lrps", "crash_reason")
		testTableAndColumn("actual_lrps", "placement_error")

		It("does not change the default", func() {
			query := helpers.RebindForFlavor("insert into actual_lrps(crash_reason) values(?)", flavor)
			_, err := rawSQLDB.Exec(query, "crash_reason")
			Expect(err).NotTo(HaveOccurred())
			row := rawSQLDB.QueryRow("select placement_error from actual_lrps")
			Expect(err).NotTo(HaveOccurred())
			var actualValue string
			Expect(row.Scan(&actualValue)).To(Succeed())
			Expect(actualValue).To(Equal(""))
		})

		It("does not remove non null constraint", func() {
			query := helpers.RebindForFlavor("insert into actual_lrps(crash_reason) values(?)", flavor)
			_, err := rawSQLDB.Exec(query, nil)
			Expect(err).To(MatchError(ContainSubstring("null")))
		})
	})

	Describe("Down", func() {
		It("returns a not implemented error", func() {
			Expect(migration.Down(logger)).To(HaveOccurred())
		})
	})
})
