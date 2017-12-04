package encryptor_test

import (
	"crypto/rand"
	"errors"

	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/encryptor"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Encryptor", func() {
	var (
		runner           ifrit.Runner
		encryptorProcess ifrit.Process

		logger     *lagertest.TestLogger
		cryptor    encryption.Cryptor
		keyManager encryption.KeyManager

		fakeDB *dbfakes.FakeEncryptionDB

		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		fakeMetronClient = new(mfakes.FakeIngressClient)
		fakeDB = new(dbfakes.FakeEncryptionDB)

		logger = lagertest.NewTestLogger("test")

		oldKey, err := encryption.NewKey("old-key", "old-passphrase")
		encryptionKey, err := encryption.NewKey("label", "passphrase")
		Expect(err).NotTo(HaveOccurred())
		keyManager, err = encryption.NewKeyManager(encryptionKey, []encryption.Key{oldKey})
		Expect(err).NotTo(HaveOccurred())
		cryptor = encryption.NewCryptor(keyManager, rand.Reader)

		fakeDB.EncryptionKeyLabelReturns("", models.ErrResourceNotFound)
	})

	JustBeforeEach(func() {
		runner = encryptor.New(logger, fakeDB, keyManager, cryptor, clock.NewClock(), fakeMetronClient)
		encryptorProcess = ifrit.Background(runner)
	})

	AfterEach(func() {
		ginkgomon.Kill(encryptorProcess)
	})

	It("reports the duration that it took to encrypt", func() {
		Eventually(encryptorProcess.Ready()).Should(BeClosed())
		Eventually(logger.LogMessages).Should(ContainElement("test.encryptor.encryption-finished"))

		Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
		name, duration := fakeMetronClient.SendDurationArgsForCall(0)
		Expect(name).To(Equal("EncryptionDuration"))
		Expect(duration).NotTo(BeZero())
	})

	Context("when there is no current encryption key", func() {
		BeforeEach(func() {
			fakeDB.EncryptionKeyLabelReturns("", models.ErrResourceNotFound)
		})

		It("encrypts all the existing records", func() {
			Eventually(encryptorProcess.Ready()).Should(BeClosed())
			Eventually(logger.LogMessages).Should(ContainElement("test.encryptor.encryption-finished"))
			Expect(fakeDB.PerformEncryptionCallCount()).To(Equal(1))
		})

		It("writes the current encryption key", func() {
			Eventually(fakeDB.SetEncryptionKeyLabelCallCount).Should(Equal(1))
			_, newLabel := fakeDB.SetEncryptionKeyLabelArgsForCall(0)
			Expect(newLabel).To(Equal("label"))
		})
	})

	Context("when encrypting fails", func() {
		BeforeEach(func() {
			fakeDB.PerformEncryptionReturns(errors.New("something is broken"))
		})

		It("does not fail and logs the error", func() {
			Eventually(encryptorProcess.Ready()).Should(BeClosed())
			Eventually(logger.LogMessages).Should(ContainElement("test.encryptor.encryption-finished"))

			Expect(logger.LogMessages()).To(ContainElement("test.encryptor.encryption-failed"))
		})

		It("does not change the key in the db", func() {
			Consistently(fakeDB.SetEncryptionKeyLabelCallCount).Should(Equal(0))
		})
	})

	Context("when fetching the current encryption key fails", func() {
		BeforeEach(func() {
			fakeDB.EncryptionKeyLabelReturns("", errors.New("can't fetch"))
		})

		It("fails early", func() {
			var err error
			Eventually(encryptorProcess.Wait()).Should(Receive(&err))
			Expect(err).To(HaveOccurred())
			Expect(encryptorProcess.Ready()).ToNot(BeClosed())
		})
	})

	Context("when the current encryption key is not known to the encryptor", func() {
		BeforeEach(func() {
			fakeDB.EncryptionKeyLabelReturns("some-unknown-key", nil)
		})

		It("shuts down wihtout signalling ready", func() {
			var err error
			Eventually(encryptorProcess.Wait()).Should(Receive(&err))
			Expect(err).To(MatchError("Existing encryption key version (some-unknown-key) is not among the known keys"))
			Expect(encryptorProcess.Ready()).ToNot(BeClosed())
		})

		It("does not change the key in the db", func() {
			Consistently(fakeDB.SetEncryptionKeyLabelCallCount).Should(Equal(0))
		})
	})

	Context("when the current encryption key is the same as the encryptor's encryption key", func() {
		BeforeEach(func() {
			fakeDB.EncryptionKeyLabelReturns("label", nil)
		})

		It("signals ready and does not change the version", func() {
			Eventually(encryptorProcess.Ready()).Should(BeClosed())
			Consistently(fakeDB.SetEncryptionKeyLabelCallCount).Should(Equal(0))
		})
	})

	Context("when the current encryption key is one of the encryptor's decryption keys", func() {
		BeforeEach(func() {
			fakeDB.EncryptionKeyLabelReturns("old-key", nil)
		})

		It("encrypts all the existing records", func() {
			Eventually(encryptorProcess.Ready()).Should(BeClosed())
			Eventually(logger.LogMessages).Should(ContainElement("test.encryptor.encryption-finished"))

			Expect(fakeDB.PerformEncryptionCallCount()).To(Equal(1))
		})

		It("writes the current encryption key", func() {
			Eventually(fakeDB.SetEncryptionKeyLabelCallCount).Should(Equal(1))
			_, newLabel := fakeDB.SetEncryptionKeyLabelArgsForCall(0)
			Expect(newLabel).To(Equal("label"))
		})
	})
})
