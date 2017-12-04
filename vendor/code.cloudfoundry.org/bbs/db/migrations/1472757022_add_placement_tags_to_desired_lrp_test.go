package migrations_test

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add Placement Tags to Desired LRPs", func() {
	var (
		mig       migration.Migration
		migErr    error
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		mig = migrations.NewAddPlacementTagsToDesiredLRPs()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.Migrations).To(ContainElement(mig))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(mig.Version()).To(BeEquivalentTo(1472757022))
		})
	})

	Describe("Up", func() {
		var initialMigrations migration.Migrations

		BeforeEach(func() {
			initialMigrations = []migration.Migration{
				migrations.NewETCDToSQL(),
				migrations.NewIncreaseRunInfoColumnSize(),
			}

			for _, m := range initialMigrations {
				m.SetRawSQLDB(rawSQLDB)
				m.SetDBFlavor(flavor)
				m.SetClock(fakeClock)
				err := m.Up(logger)
				Expect(err).NotTo(HaveOccurred())
			}

			// Can't do this in the Describe BeforeEach
			// as the test on line 37 will cause ginkgo to panic
			mig.SetRawSQLDB(rawSQLDB)
			mig.SetDBFlavor(flavor)
		})

		JustBeforeEach(func() {
			migErr = mig.Up(logger)
		})

		It("does not error out", func() {
			Expect(migErr).NotTo(HaveOccurred())
		})

		It("should add a placement_tags column to desired lrps", func() {
			placementTags := []string{"tag-1"}

			jsonData, err := json.Marshal(placementTags)
			Expect(err).NotTo(HaveOccurred())

			_, err = rawSQLDB.Exec(
				helpers.RebindForFlavor(
					`INSERT INTO desired_lrps
						  (process_guid, domain, placement_tags, log_guid, instances, memory_mb,
							  disk_mb, rootfs, routes, volume_placement, modification_tag_epoch, run_info)
						  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					flavor,
				),
				"guid", "domain",
				jsonData,
				"log guid", 2, 1, 1, "rootfs", "routes", "volumes yo", 1, "run info",
			)
			Expect(err).NotTo(HaveOccurred())

			var fetchedJSONData string
			query := helpers.RebindForFlavor("select placement_tags from desired_lrps limit 1", flavor)
			row := rawSQLDB.QueryRow(query)
			Expect(row.Scan(&fetchedJSONData)).NotTo(HaveOccurred())
			Expect(fetchedJSONData).To(BeEquivalentTo(jsonData))
		})
	})

	Describe("Down", func() {
		It("returns a not implemented error", func() {
			Expect(mig.Down(logger)).To(HaveOccurred())
		})
	})
})
