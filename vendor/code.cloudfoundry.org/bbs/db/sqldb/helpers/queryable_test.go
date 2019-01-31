package helpers_test

import (
	"database/sql"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers/monitor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("SQL Helpers", func() {
	var (
		logger *lagertest.TestLogger
		helper helpers.SQLHelper
		mon    monitor.Monitor
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("query-metrics-test")
		helper = helpers.NewSQLHelper(dbFlavor)
		mon = monitor.New()

		tableName = fmt.Sprintf("dummy_%d", GinkgoParallelNode())
		tableQuery := fmt.Sprintf("CREATE TABLE %s (existingcol INT);", tableName)
		_, err := db.Exec(tableQuery)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s;", tableName))
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Transactions", func() {
		It("returns a transaction and increments metrics", func() {
			q := helpers.NewMonitoredDB(db, mon)
			err := helper.Transact(logger, q, func(l lager.Logger, tx helpers.Tx) error {
				_, err := helper.Insert(l, tx, tableName, helpers.SQLAttributes{"existingcol": 3})
				return err
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Failed()).To(BeZero())
			Expect(mon.Succeeded()).To(BeEquivalentTo(3))
			Expect(mon.Total()).To(BeEquivalentTo(3))
		})

		It("rolls back a transaction and increments metrics", func() {
			q := helpers.NewMonitoredDB(db, mon)
			err := helper.Transact(logger, q, func(l lager.Logger, tx helpers.Tx) error {
				_, err := helper.Insert(l, tx, tableName, helpers.SQLAttributes{"wrongcolumn": 3})
				return err
			})
			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
			Expect(mon.Succeeded()).To(BeEquivalentTo(2))
		})
	})

	Describe("Begin", func() {
		It("returns a transaction and increments", func() {
			q := helpers.NewMonitoredDB(db, mon)

			tx, err := q.Begin()
			defer tx.Commit()
			Expect(err).NotTo(HaveOccurred())

			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})
	})

	Describe("Insert", func() {
		It("executes queries", func() {
			q := helpers.NewMonitoredDB(db, mon)

			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3})
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)

			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"wrongcolumn": 3})
			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			m := monitor.New()
			q := helpers.NewMonitoredDB(db, m)
			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3})
			Expect(err).NotTo(HaveOccurred())
		})

		It("executes queries", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.Update(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.Update(logger, q, tableName, helpers.SQLAttributes{"wrongcolumn": 3}, "")

			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("One", func() {
		BeforeEach(func() {
			m := monitor.New()
			q := helpers.NewMonitoredDB(db, m)
			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3})
			Expect(err).NotTo(HaveOccurred())
		})

		It("executes queries", func() {
			q := helpers.NewMonitoredDB(db, mon)
			row := helper.One(logger, q, tableName, []string{"existingcol"}, false, "")
			var value int
			err := row.Scan(&value)
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})

		It("does not return an error if the row does not exist", func() {
			q := helpers.NewMonitoredDB(db, mon)
			row := helper.One(logger, q, tableName, []string{"existingcol"}, false, "existingcol = ?", 12345)
			var value int
			err := row.Scan(&value)
			Expect(err).To(MatchError(sql.ErrNoRows))
			Expect(mon.Failed()).To(BeZero())
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)
			row := helper.One(logger, q, tableName, []string{"field2"}, false, "")

			var value int
			err := row.Scan(&value)
			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("All", func() {
		BeforeEach(func() {
			m := monitor.New()
			q := helpers.NewMonitoredDB(db, m)
			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3})
			Expect(err).NotTo(HaveOccurred())
		})

		It("executes queries", func() {
			q := helpers.NewMonitoredDB(db, mon)
			rows, err := helper.All(logger, q, tableName, []string{"existingcol"}, false, "")
			defer rows.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.All(logger, q, tableName, []string{"wrongcolumn"}, false, "")
			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("Upsert", func() {
		It("executes queries", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.Upsert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(2))
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.Upsert(logger, q, tableName, helpers.SQLAttributes{"wrongcolumn": 3}, "")

			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			m := monitor.New()
			q := helpers.NewMonitoredDB(db, m)
			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3})
			Expect(err).NotTo(HaveOccurred())
		})

		It("executes queries", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.Delete(logger, q, tableName, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)
			_, err := helper.Delete(logger, q, "wrongtable", "")
			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("Count", func() {
		BeforeEach(func() {
			m := monitor.New()
			q := helpers.NewMonitoredDB(db, m)
			_, err := helper.Insert(logger, q, tableName, helpers.SQLAttributes{"existingcol": 3})
			Expect(err).NotTo(HaveOccurred())
		})

		It("executes a query", func() {
			q := helpers.NewMonitoredDB(db, mon)

			_, err := helper.Count(logger, q, tableName, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(mon.Succeeded()).To(BeEquivalentTo(1))
		})

		It("returns an error on a bad query", func() {
			q := helpers.NewMonitoredDB(db, mon)

			_, err := helper.Count(logger, q, "wrongtable", "")
			Expect(err).To(HaveOccurred())
			Expect(mon.Failed()).To(BeEquivalentTo(1))
		})
	})

	Describe("OpenConnections", func() {
		It("returns the number of open connections to the database", func() {
			q := helpers.NewMonitoredDB(db, mon)
			Expect(q.OpenConnections()).To(BeNumerically(">=", 0))
		})
	})
})
