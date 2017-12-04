package migrations_test

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	etcdclient "github.com/coreos/go-etcd/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	_ "github.com/go-sql-driver/mysql"

	"testing"
)

var (
	etcdPort    int
	etcdUrl     string
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
	etcdClient  *etcdclient.Client
	storeClient etcd.StoreClient

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

	etcdPort = 4001 + GinkgoParallelNode()
	etcdUrl = fmt.Sprintf("http://127.0.0.1:%d", etcdPort)
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)

	etcdRunner.Start()

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
	etcdRunner.Stop()

	Expect(rawSQLDB.Close()).NotTo(HaveOccurred())
	ginkgomon.Kill(sqlProcess, 5*time.Second)
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()

	etcdClient = etcdRunner.Client()
	etcdClient.SetConsistency(etcdclient.STRONG_CONSISTENCY)

	storeClient = etcd.NewStoreClient(etcdClient)

	sqlRunner.Reset()
})
