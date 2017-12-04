package encryption_test

import (
	"code.cloudfoundry.org/bbs/encryption"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KeyManager", func() {
	var (
		encryptionKey  encryption.Key
		decryptionKeys []encryption.Key
		manager        encryption.KeyManager
		cerr           error
	)

	BeforeEach(func() {
		var err error
		encryptionKey, err = encryption.NewKey("key label", "pass phrase")
		Expect(err).NotTo(HaveOccurred())
		decryptionKeys = []encryption.Key{}
		cerr = nil
	})

	JustBeforeEach(func() {
		manager, cerr = encryption.NewKeyManager(encryptionKey, decryptionKeys)
	})

	It("stores the correct encryption key", func() {
		Expect(cerr).NotTo(HaveOccurred())
		Expect(manager.EncryptionKey()).To(Equal(encryptionKey))
	})

	It("adds the encryption key as a decryption key", func() {
		Expect(manager.DecryptionKey(encryptionKey.Label())).To(Equal(manager.EncryptionKey()))
	})

	Context("when decryption keys are provided", func() {
		BeforeEach(func() {
			key1, err := encryption.NewKey("label1", "some pass phrase")
			Expect(err).NotTo(HaveOccurred())

			key2, err := encryption.NewKey("label2", "some other pass phrase")
			Expect(err).NotTo(HaveOccurred())

			key3, err := encryption.NewKey("label3", "")
			Expect(err).NotTo(HaveOccurred())

			decryptionKeys = []encryption.Key{key1, key2, key3}
		})

		It("stores the decryption keys", func() {
			for _, decryptionKey := range decryptionKeys {
				key := manager.DecryptionKey(decryptionKey.Label())
				Expect(key).To(Equal(decryptionKey))
			}
		})

		It("includes the encryption key", func() {
			key := manager.DecryptionKey(encryptionKey.Label())
			Expect(key).To(Equal(manager.EncryptionKey()))
		})
	})

	Context("when the encryption key and a decryption key have the same label but different blocks", func() {
		BeforeEach(func() {
			decryptKey, err := encryption.NewKey("key label", "a different pass phrase")
			Expect(err).NotTo(HaveOccurred())

			decryptionKeys = []encryption.Key{decryptKey}
		})

		It("returns a multiple key error", func() {
			Expect(cerr).To(MatchError(`Multiple keys with the same label: "key label"`))
		})
	})

	Context("when different keys with the same label are provided in decryption keys", func() {
		BeforeEach(func() {
			duplicateLabelKey1, err := encryption.NewKey("label", "a pass phrase")
			duplicateLabelKey2, err := encryption.NewKey("label", "a different pass phrase")
			Expect(err).NotTo(HaveOccurred())

			decryptionKeys = []encryption.Key{duplicateLabelKey1, duplicateLabelKey2}
		})

		It("returns a multiple key error", func() {
			Expect(cerr).To(MatchError(`Multiple keys with the same label: "label"`))
		})
	})

	Context("when attempting to retrieve a key that does not exist", func() {
		It("returns nil", func() {
			key := manager.DecryptionKey("bogus")
			Expect(key).To(BeNil())
		})
	})

	Context("when no decryption keys are provided", func() {
		BeforeEach(func() {
			decryptionKeys = nil
		})

		It("still includes the encryption key", func() {
			key := manager.DecryptionKey(encryptionKey.Label())
			Expect(key).To(Equal(encryptionKey))
		})
	})
})
