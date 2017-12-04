package etcd_test

import (
	"crypto/rand"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/db/etcd/fakes"
	"code.cloudfoundry.org/bbs/db/etcd/test/etcd_helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	etcdclient "github.com/coreos/go-etcd/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

const DesiredLRPCreationTimeout = time.Minute

var (
	etcdPort         int
	etcdUrl          string
	etcdRunner       *etcdstorerunner.ETCDClusterRunner
	storeClient      etcd.StoreClient
	fakeStoreClient  *fakes.FakeStoreClient
	fakeMetronClient *mfakes.FakeIngressClient

	logger     *lagertest.TestLogger
	clock      *fakeclock.FakeClock
	etcdHelper *etcd_helpers.ETCDHelper

	etcdDB              db.DB
	etcdDBWithFakeStore db.DB
	workPoolCreateError error

	cryptor encryption.Cryptor
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ETCD DB Suite")
}

var _ = BeforeSuite(func() {
	clock = fakeclock.NewFakeClock(time.Unix(0, 1138))

	etcdPort = 4001 + GinkgoParallelNode()
	etcdUrl = fmt.Sprintf("http://127.0.0.1:%d", etcdPort)
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)

	etcdRunner.Start()

	Expect(workPoolCreateError).ToNot(HaveOccurred())

	encryptionKey, err := encryption.NewKey("label", "passphrase")
	Expect(err).NotTo(HaveOccurred())
	keyManager, err := encryption.NewKeyManager(encryptionKey, nil)
	Expect(err).NotTo(HaveOccurred())
	cryptor = encryption.NewCryptor(keyManager, rand.Reader)
})

var _ = AfterSuite(func() {
	etcdRunner.Stop()
})

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("test")

	etcdRunner.Reset()
	fakeMetronClient = new(mfakes.FakeIngressClient)

	etcdClient := etcdRunner.Client()
	etcdClient.SetConsistency(etcdclient.STRONG_CONSISTENCY)
	storeClient = etcd.NewStoreClient(etcdClient)
	fakeStoreClient = &fakes.FakeStoreClient{}
	etcdHelper = etcd_helpers.NewETCDHelper(format.ENCRYPTED_PROTO, cryptor, storeClient, clock)
	etcdDB = etcd.NewETCD(format.ENCRYPTED_PROTO, 100, 100, DesiredLRPCreationTimeout, cryptor, storeClient, clock, fakeMetronClient)
	etcdDBWithFakeStore = etcd.NewETCD(format.ENCRYPTED_PROTO, 100, 100, DesiredLRPCreationTimeout, cryptor, fakeStoreClient, clock, fakeMetronClient)
})
