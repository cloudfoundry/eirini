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

var _ = Describe("AddTaskRejectionCount", func() {
	var (
		mig migration.Migration
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		mig = migrations.NewAddTaskRejectionCount()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(mig))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(mig.Version()).To(BeEquivalentTo(1522700600))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			initialMigration := migrations.NewInitSQL()
			initialMigration.SetRawSQLDB(rawSQLDB)
			initialMigration.SetDBFlavor(flavor)
			initialMigration.SetClock(fakeClock)
			Expect(initialMigration.Up(logger)).To(Succeed())

			mig.SetRawSQLDB(rawSQLDB)
			mig.SetDBFlavor(flavor)

		})

		It("adds rejection_count to tasks and defaults it to 0", func() {
			Expect(mig.Up(logger)).To(Succeed())

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

			var rejectionCount int
			query := helpers.RebindForFlavor("select rejection_count from tasks limit 1", flavor)
			row := rawSQLDB.QueryRow(query)
			Expect(row.Scan(&rejectionCount)).NotTo(HaveOccurred())
			Expect(rejectionCount).To(Equal(0))
		})

		It("is idempotent", func() {
			testIdempotency(rawSQLDB, mig, logger)
		})
	})
})
