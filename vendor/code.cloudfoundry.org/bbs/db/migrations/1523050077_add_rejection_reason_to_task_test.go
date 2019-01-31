package migrations_test

import (
	"time"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddRejectionReasonToTask", func() {
	var (
		migration migration.Migration
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		rawSQLDB.Exec("DROP TABLE tasks;")

		migration = migrations.NewAddRejectionReasonToTask()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1523050077))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			initialMigration := migrations.NewInitSQL()
			initialMigration.SetRawSQLDB(rawSQLDB)
			initialMigration.SetDBFlavor(flavor)
			initialMigration.SetClock(fakeClock)
			Expect(initialMigration.Up(logger)).To(Succeed())

			migration.SetRawSQLDB(rawSQLDB)
			migration.SetDBFlavor(flavor)
		})

		It("add rejection_reason to the tasks and defaults it to \"\"", func() {
			Expect(migration.Up(logger)).To(Succeed())

			_, err := rawSQLDB.Exec(
				helpers.RebindForFlavor(
					`INSERT INTO tasks
						  (guid, domain, task_definition)
						  VALUES (?, ?, ?)`,
					flavor,
				),
				"guid", "domain", "task_definition",
			)
			Expect(err).NotTo(HaveOccurred())

			var rejectionReason string
			query := helpers.RebindForFlavor("select rejection_reason from tasks limit 1", flavor)
			row := rawSQLDB.QueryRow(query)
			Expect(row.Scan(&rejectionReason)).To(Succeed())
			Expect(rejectionReason).To(Equal(""))
		})

		It("is idempotent", func() {
			testIdempotency(rawSQLDB, migration, logger)
		})
	})
})
