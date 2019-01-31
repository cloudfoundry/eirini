package migrations_test

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/migration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Init SQL Migration", func() {
	var (
		migration    migration.Migration
		migrationErr error
	)

	BeforeEach(func() {
		migration = migrations.NewInitSQL()

		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1461790966))
		})
	})

	Describe("Up", func() {
		JustBeforeEach(func() {
			migration.SetRawSQLDB(rawSQLDB)
			migration.SetCryptor(cryptor)
			migration.SetClock(fakeClock)
			migration.SetDBFlavor(flavor)
			migrationErr = migration.Up(logger)
		})

		Context("when there is existing data in the database", func() {
			BeforeEach(func() {
				var err error

				_, err = rawSQLDB.Exec(`CREATE TABLE domains( domain VARCHAR(255) PRIMARY KEY);`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`CREATE TABLE desired_lrps( process_guid VARCHAR(255) PRIMARY KEY);`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`CREATE TABLE actual_lrps( process_guid VARCHAR(255) PRIMARY KEY);`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`CREATE TABLE tasks( guid VARCHAR(255) PRIMARY KEY);`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`INSERT INTO domains VALUES ('test-domain')`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`INSERT INTO desired_lrps VALUES ('test-guid')`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`INSERT INTO actual_lrps VALUES ('test-guid')`)
				Expect(err).NotTo(HaveOccurred())

				_, err = rawSQLDB.Exec(`INSERT INTO tasks VALUES ('test-guid')`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should truncate the tables and start migration", func() {
				var value string
				err := rawSQLDB.QueryRow(`SELECT domain FROM domains WHERE domain='test-domain'`).Scan(&value)
				Expect(err).To(MatchError(sql.ErrNoRows))
			})

			It("should truncate desired_lrps table", func() {
				var value string
				err := rawSQLDB.QueryRow(`SELECT process_guid FROM desired_lrps WHERE process_guid='test-guid'`).Scan(&value)
				Expect(err).To(MatchError(sql.ErrNoRows))
			})

			It("should truncate actual_lrps table", func() {
				var value string
				err := rawSQLDB.QueryRow(`SELECT process_guid FROM actual_lrps WHERE process_guid='test-guid'`).Scan(&value)
				Expect(err).To(MatchError(sql.ErrNoRows))
			})

			It("should truncate tasks table", func() {
				var value string
				err := rawSQLDB.QueryRow(`SELECT guid FROM tasks WHERE guid='test-guid'`).Scan(&value)
				Expect(err).To(MatchError(sql.ErrNoRows))
			})
		})

		Context("when some tables exist", func() {
			BeforeEach(func() {
				var err error

				_, err = rawSQLDB.Exec(`CREATE TABLE tasks( guid VARCHAR(255) PRIMARY KEY);`)
				Expect(err).NotTo(HaveOccurred())
				_, err = rawSQLDB.Exec(`INSERT INTO tasks VALUES ('test-guid')`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should drop those tables", func() {
				var value string
				err := rawSQLDB.QueryRow(`SELECT guid FROM tasks WHERE guid='test-guid'`).Scan(&value)
				Expect(err).To(MatchError(sql.ErrNoRows))
			})
		})

		Context("when no tables exist", func() {
			It("creates the sql schema and returns", func() {
				Expect(migrationErr).NotTo(HaveOccurred())
				rows, err := rawSQLDB.Query(`SELECT table_name FROM information_schema.tables`)
				Expect(err).NotTo(HaveOccurred())
				defer rows.Close()

				tables := []string{}
				for rows.Next() {
					var tableName string
					err := rows.Scan(&tableName)
					Expect(err).NotTo(HaveOccurred())
					tables = append(tables, tableName)
				}
				Expect(tables).To(ContainElement("domains"))
				Expect(tables).To(ContainElement("desired_lrps"))
				Expect(tables).To(ContainElement("actual_lrps"))
				Expect(tables).To(ContainElement("tasks"))
			})
		})
	})
})
