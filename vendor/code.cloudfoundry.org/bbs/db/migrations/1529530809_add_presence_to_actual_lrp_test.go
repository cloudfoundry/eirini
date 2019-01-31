package migrations_test

import (
	"fmt"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddPresenceToActualLrp", func() {
	var (
		migration migration.Migration
	)

	BeforeEach(func() {
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		migration = migrations.NewAddPresenceToActualLrp()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1529530809))
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

		It("add presence to the actual_lrps and defaults it to ordinary", func() {
			Expect(migration.Up(logger)).To(Succeed())

			_, err := rawSQLDB.Exec(
				helpers.RebindForFlavor(
					`INSERT INTO actual_lrps
						(process_guid, instance_index, domain, state, net_info,
						modification_tag_epoch, modification_tag_index)
					VALUES (?, ?, ?, ?, ?, ?, ?)`,
					flavor,
				),
				"guid", 10, "cfapps", "RUNNING", "", "epoch", 0,
			)
			Expect(err).NotTo(HaveOccurred())

			var presence string
			query := helpers.RebindForFlavor("SELECT presence FROM actual_lrps LIMIT 1", flavor)
			row := rawSQLDB.QueryRow(query)
			Expect(row.Scan(&presence)).To(Succeed())
			Expect(presence).To(Equal(fmt.Sprintf("%d", models.ActualLRP_Ordinary)))
		})

		It("adds presence as a primary key so that duplicate entries with different presence do not violate the unique constraint", func() {
			Expect(migration.Up(logger)).To(Succeed())

			_, err := rawSQLDB.Exec(
				helpers.RebindForFlavor(
					`INSERT INTO actual_lrps
						(process_guid, instance_index, domain, state, net_info,
						modification_tag_epoch, modification_tag_index)
					VALUES (?, ?, ?, ?, ?, ?, ?)`,
					flavor,
				),
				"guid", 10, "cfapps", "RUNNING", "", "epoch", 0,
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = rawSQLDB.Exec(
				helpers.RebindForFlavor(
					`INSERT INTO actual_lrps
						(process_guid, instance_index, domain, state, net_info,
						modification_tag_epoch, modification_tag_index, presence)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					flavor,
				),
				"guid", 10, "cfapps", "RUNNING", "", "epoch", 0, models.ActualLRP_Evacuating,
			)
			Expect(err).NotTo(HaveOccurred())

		})

		Context("with preexisting data with evacuating set to true", func() {
			BeforeEach(func() {
				_, err := rawSQLDB.Exec(
					helpers.RebindForFlavor(
						`INSERT INTO actual_lrps
						(process_guid, instance_index, domain, state, net_info,
						modification_tag_epoch, modification_tag_index, evacuating)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
						flavor,
					),
					"guid", 1, "cfapps", "RUNNING", "", "epoch", 0, true,
				)
				Expect(err).NotTo(HaveOccurred())

			})

			It("does not error on LRPs with the same process_guid + index when changing the primary key", func() {
				_, err := rawSQLDB.Exec(
					helpers.RebindForFlavor(
						`INSERT INTO actual_lrps
						(process_guid, instance_index, domain, state, net_info,
						modification_tag_epoch, modification_tag_index, evacuating)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
						flavor,
					),
					"guid", 1, "cfapps", "RUNNING", "", "epoch", 0, false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(migration.Up(logger)).To(Succeed())
			})

			It("sets the presence of the evacuating row to evacuating", func() {
				Expect(migration.Up(logger)).To(Succeed())

				var presence string
				query := helpers.RebindForFlavor("SELECT presence FROM actual_lrps WHERE evacuating = true LIMIT 1", flavor)
				row := rawSQLDB.QueryRow(query)
				Expect(row.Scan(&presence)).To(Succeed())
				Expect(presence).To(Equal(fmt.Sprintf("%d", models.ActualLRP_Evacuating)))
			})

			It("is idempotent even with preexisting data", func() {
				testIdempotency(rawSQLDB, migration, logger)
			})
		})
	})
})
