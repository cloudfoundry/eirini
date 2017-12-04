package sqldb_test

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"time"

	thepackagedb "code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/guidprovider/guidproviderfakes"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	_ "github.com/lib/pq"

	"testing"
)

var (
	db                                   *sql.DB
	sqlDB                                *sqldb.SQLDB
	fakeClock                            *fakeclock.FakeClock
	fakeGUIDProvider                     *guidproviderfakes.FakeGUIDProvider
	logger                               *lagertest.TestLogger
	cryptor                              encryption.Cryptor
	serializer                           format.Serializer
	migrationProcess                     ifrit.Process
	dbDriverName, dbBaseConnectionString string
	dbFlavor                             string
	fakeMetronClient                     *mfakes.FakeIngressClient
)

func TestSql(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "SQL DB Suite")
}

var _ = BeforeSuite(func() {
	var err error
	fakeClock = fakeclock.NewFakeClock(time.Now())
	fakeGUIDProvider = &guidproviderfakes.FakeGUIDProvider{}
	fakeMetronClient = new(mfakes.FakeIngressClient)
	logger = lagertest.NewTestLogger("sql-db")

	if test_helpers.UsePostgres() {
		dbDriverName = "postgres"
		dbBaseConnectionString = "postgres://diego:diego_pw@localhost/"
		dbFlavor = helpers.Postgres
	} else if test_helpers.UseMySQL() {
		dbDriverName = "mysql"
		dbBaseConnectionString = "diego:diego_password@/"
		dbFlavor = helpers.MySQL
	} else {
		panic("Unsupported driver")
	}

	// mysql must be set up on localhost as described in the CONTRIBUTING.md doc
	// in diego-release.
	db, err = sql.Open(dbDriverName, dbBaseConnectionString)
	Expect(err).NotTo(HaveOccurred())
	Expect(db.Ping()).NotTo(HaveOccurred())

	// Ensure that if another test failed to clean up we can still proceed
	db.Exec(fmt.Sprintf("DROP DATABASE diego_%d", GinkgoParallelNode()))

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE diego_%d", GinkgoParallelNode()))
	Expect(err).NotTo(HaveOccurred())

	db, err = sql.Open(dbDriverName, fmt.Sprintf("%sdiego_%d", dbBaseConnectionString, GinkgoParallelNode()))
	Expect(err).NotTo(HaveOccurred())
	Expect(db.Ping()).NotTo(HaveOccurred())

	encryptionKey, err := encryption.NewKey("label", "passphrase")
	Expect(err).NotTo(HaveOccurred())
	keyManager, err := encryption.NewKeyManager(encryptionKey, nil)
	Expect(err).NotTo(HaveOccurred())
	cryptor = encryption.NewCryptor(keyManager, rand.Reader)
	serializer = format.NewSerializer(cryptor)

	sqlDB = sqldb.NewSQLDB(db, 5, 5, format.ENCRYPTED_PROTO, cryptor, fakeGUIDProvider, fakeClock, dbFlavor, fakeMetronClient)
	err = sqlDB.CreateConfigurationsTable(logger)
	if err != nil {
		logger.Fatal("sql-failed-create-configurations-table", err)
	}

	// ensures sqlDB matches the db.DB interface
	var _ thepackagedb.DB = sqlDB
})

var _ = BeforeEach(func() {
	fakeMetronClient = new(mfakes.FakeIngressClient)
	migrationMetronClient := new(mfakes.FakeIngressClient)
	sqlDB = sqldb.NewSQLDB(db, 5, 5, format.ENCRYPTED_PROTO, cryptor, fakeGUIDProvider, fakeClock, dbFlavor, fakeMetronClient)

	migrationsDone := make(chan struct{})

	migrationManager := migration.NewManager(logger,
		nil,
		nil,
		sqlDB,
		db,
		cryptor,
		migrations.Migrations,
		migrationsDone,
		fakeClock,
		dbDriverName,
		migrationMetronClient,
	)

	migrationProcess = ifrit.Invoke(migrationManager)

	Consistently(migrationProcess.Wait()).ShouldNot(Receive())
	Eventually(migrationsDone).Should(BeClosed())

	// ensure that all sqldb functions being tested only require one connection
	// to operate, otherwise a deadlock can be caused in bbs. For more
	// information see https://www.pivotaltracker.com/story/show/136754083
	db.SetMaxOpenConns(1)
})

var _ = AfterEach(func() {
	fakeGUIDProvider.NextGUIDReturns("", nil)
	truncateTables(db)
})

var _ = AfterSuite(func() {
	if migrationProcess != nil {
		migrationProcess.Signal(os.Kill)
	}

	Expect(db.Close()).NotTo(HaveOccurred())
	db, err := sql.Open(dbDriverName, dbBaseConnectionString)
	Expect(err).NotTo(HaveOccurred())
	Expect(db.Ping()).NotTo(HaveOccurred())
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE diego_%d", GinkgoParallelNode()))
	Expect(err).NotTo(HaveOccurred())
	Expect(db.Close()).NotTo(HaveOccurred())
})

func truncateTables(db *sql.DB) {
	for _, query := range truncateTablesSQL {
		result, err := db.Exec(query)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RowsAffected()).To(BeEquivalentTo(0))
	}
}

var truncateTablesSQL = []string{
	"TRUNCATE TABLE domains",
	"TRUNCATE TABLE tasks",
	"TRUNCATE TABLE desired_lrps",
	"TRUNCATE TABLE actual_lrps",
	"TRUNCATE TABLE configurations",
}

func randStr(strSize int) string {
	alphanum := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, strSize)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}
