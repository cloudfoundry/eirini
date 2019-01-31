package migrations_test

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	_ "github.com/go-sql-driver/mysql"

	"testing"
)

var (
	flavor string

	rawSQLDB   *sql.DB
	sqlProcess ifrit.Process
	sqlRunner  sqlrunner.SQLRunner

	cryptor   encryption.Cryptor
	fakeClock *fakeclock.FakeClock
	logger    *lagertest.TestLogger
)

func TestMigrations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migrations Suite")
}

var _ = BeforeSuite(func() {
	logger = lagertest.NewTestLogger("test")

	dbName := fmt.Sprintf("diego_%d", GinkgoParallelNode())
	sqlRunner = test_helpers.NewSQLRunner(dbName)
	sqlProcess = ginkgomon.Invoke(sqlRunner)

	// mysql must be set up on localhost as described in the CONTRIBUTING.md doc
	// in diego-release.
	var err error
	rawSQLDB, err = sql.Open(sqlRunner.DriverName(), sqlRunner.ConnectionString())
	Expect(err).NotTo(HaveOccurred())
	Expect(rawSQLDB.Ping()).NotTo(HaveOccurred())

	flavor = sqlRunner.DriverName()

	encryptionKey, err := encryption.NewKey("label", "passphrase")
	Expect(err).NotTo(HaveOccurred())
	keyManager, err := encryption.NewKeyManager(encryptionKey, nil)
	Expect(err).NotTo(HaveOccurred())
	cryptor = encryption.NewCryptor(keyManager, rand.Reader)

	fakeClock = fakeclock.NewFakeClock(time.Now())
})

var _ = AfterSuite(func() {
	Expect(rawSQLDB.Close()).NotTo(HaveOccurred())
	ginkgomon.Kill(sqlProcess, 5*time.Second)
})

var _ = BeforeEach(func() {
	sqlRunner.Reset()
})

func listTableNames(db *sql.DB) []string {
	var rows *sql.Rows
	var err error
	switch flavor {
	case "mysql":
		rows, err = db.Query("SHOW TABLES")
	case "postgres":
		rows, err = db.Query("SELECT tablename FROM pg_catalog.pg_tables;")
	default:
		Expect(flavor).To(Equal("not supported"))
	}
	Expect(err).NotTo(HaveOccurred())
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		Expect(rows.Scan(&name)).To(Succeed())
		if strings.HasPrefix(name, "pg_") || strings.HasPrefix(name, "sql_") {
			continue
		}
		tableNames = append(tableNames, name)
	}
	Expect(rows.Err()).To(Succeed())
	return tableNames
}

func getTableSchema(db *sql.DB, tableName string) []*sql.ColumnType {
	rows, err := db.Query("SELECT * FROM " + tableName)
	Expect(err).NotTo(HaveOccurred())
	columnTypes, err := rows.ColumnTypes()
	Expect(err).NotTo(HaveOccurred())
	rows.Close()
	return columnTypes
}

func getAllSchemas(db *sql.DB) ([]string, map[string][]*sql.ColumnType) {
	tableNames := listTableNames(db)
	allSchemas := make(map[string][]*sql.ColumnType)
	for _, table := range tableNames {
		allSchemas[table] = getTableSchema(db, table)
	}
	return tableNames, allSchemas
}

func dumpTableData(db *sql.DB, name string) [][][]byte {
	rows, err := db.Query("SELECT * FROM " + name)
	Expect(err).NotTo(HaveOccurred())
	defer rows.Close()

	cols, err := rows.Columns()
	Expect(err).NotTo(HaveOccurred())

	var all [][][]byte // row | col | data
	for rows.Next() {
		row := make([][]byte, len(cols))
		// create an interface slice pointing to
		// the values stored in byte slice row.
		p := make([]interface{}, len(row))
		for i := range row {
			p[i] = &row[i]
		}
		Expect(rows.Scan(p...)).To(Succeed())
		all = append(all, row)
	}
	Expect(rows.Err()).To(Succeed())

	return all
}

func testIdempotency(db *sql.DB, mig migration.Migration, logger lager.Logger) {
	Expect(mig.Up(logger)).To(Succeed())

	tableNamesBefore, allSchemasBefore := getAllSchemas(db)

	dataBefore := make(map[string][][][]byte)
	for _, name := range tableNamesBefore {
		dataBefore[name] = dumpTableData(db, name)
	}

	// some migrations will not apply a second time, but we still want
	// to make sure the data was not changed.
	mig.Up(logger)

	tableNamesAfter, allSchemasAfter := getAllSchemas(db)

	dataAfter := make(map[string][][][]byte)
	for _, name := range tableNamesAfter {
		dataAfter[name] = dumpTableData(db, name)
	}

	Expect(tableNamesBefore).To(Equal(tableNamesAfter))
	Expect(allSchemasBefore).To(Equal(allSchemasAfter))
	Expect(dataBefore).To(Equal(dataAfter))
}
