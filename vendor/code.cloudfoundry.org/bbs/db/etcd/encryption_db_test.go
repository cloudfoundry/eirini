package etcd_test

import (
	"crypto/rand"
	"fmt"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encryption", func() {
	BeforeEach(func() {
		fakeMetronClient = new(mfakes.FakeIngressClient)
	})
	Describe("SetEncryptionKeyLabel", func() {
		It("sets the encryption key label into the database", func() {
			err := etcdDB.SetEncryptionKeyLabel(logger, "expected-key")
			Expect(err).NotTo(HaveOccurred())

			response, err := storeClient.Get(etcd.EncryptionKeyLabelKey, false, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Node.Value).To(Equal("expected-key"))
		})
	})

	Describe("EncryptionKeyLabel", func() {
		Context("when the encription key label key exists", func() {
			It("retrieves the encrption key label from the database", func() {
				_, err := storeClient.Set(etcd.EncryptionKeyLabelKey, []byte("expected-key"), etcd.NO_TTL)
				Expect(err).NotTo(HaveOccurred())

				keyLabel, err := etcdDB.EncryptionKeyLabel(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(keyLabel).To(Equal("expected-key"))
			})
		})

		Context("when the encryption key label key does not exist", func() {
			It("returns a ErrResourceNotFound", func() {
				keyLabel, err := etcdDB.EncryptionKeyLabel(logger)
				Expect(err).To(MatchError(models.ErrResourceNotFound))
				Expect(keyLabel).To(Equal(""))
			})
		})
	})

	makeCryptor := func(activeLabel string, decryptionLabels ...string) encryption.Cryptor {
		activeKey, err := encryption.NewKey(activeLabel, fmt.Sprintf("%s-passphrase", activeLabel))
		Expect(err).NotTo(HaveOccurred())

		decryptionKeys := []encryption.Key{}
		for _, label := range decryptionLabels {
			key, err := encryption.NewKey(label, fmt.Sprintf("%s-passphrase", label))
			Expect(err).NotTo(HaveOccurred())
			decryptionKeys = append(decryptionKeys, key)
		}
		if len(decryptionKeys) == 0 {
			decryptionKeys = nil
		}

		keyManager, err := encryption.NewKeyManager(activeKey, decryptionKeys)
		Expect(err).NotTo(HaveOccurred())
		return encryption.NewCryptor(keyManager, rand.Reader)
	}

	Describe("PerformEncryption", func() {
		It("recursively re-encrypts all existing records", func() {
			var cryptor encryption.Cryptor
			var encoder format.Encoder

			value1 := []byte("some text")
			value2 := []byte("more text")

			cryptor = makeCryptor("old")
			encoder = format.NewEncoder(cryptor)

			encoded1, err := encoder.Encode(format.BASE64_ENCRYPTED, value1)
			Expect(err).NotTo(HaveOccurred())

			encoded2, err := encoder.Encode(format.LEGACY_UNENCODED, value2)
			Expect(err).NotTo(HaveOccurred())

			_, err = storeClient.Set(fmt.Sprintf("%s/my/key-1", etcd.V1SchemaRoot), encoded1, etcd.NO_TTL)
			Expect(err).NotTo(HaveOccurred())
			_, err = storeClient.Set(fmt.Sprintf("%s/my/nested/key-2", etcd.V1SchemaRoot), encoded2, etcd.NO_TTL)
			Expect(err).NotTo(HaveOccurred())

			cryptor = makeCryptor("new", "old")

			etcdDB = etcd.NewETCD(format.ENCRYPTED_PROTO, 100, 100, DesiredLRPCreationTimeout, cryptor, storeClient, clock, fakeMetronClient)
			err = etcdDB.PerformEncryption(logger)
			Expect(err).NotTo(HaveOccurred())

			cryptor = makeCryptor("new")
			encoder = format.NewEncoder(cryptor)

			res, err := storeClient.Get(fmt.Sprintf("%s/my/key-1", etcd.V1SchemaRoot), false, false)
			Expect(err).NotTo(HaveOccurred())
			decrypted1, err := encoder.Decode([]byte(res.Node.Value))
			Expect(err).NotTo(HaveOccurred())
			Expect(decrypted1).To(Equal(value1))

			res, err = storeClient.Get(fmt.Sprintf("%s/my/nested/key-2", etcd.V1SchemaRoot), false, false)
			Expect(err).NotTo(HaveOccurred())
			decrypted2, err := encoder.Decode([]byte(res.Node.Value))
			Expect(err).NotTo(HaveOccurred())
			Expect(decrypted2).To(Equal(value2))
		})

		It("does not fail encryption if it can't read a record", func() {
			var cryptor encryption.Cryptor
			var encoder format.Encoder

			value1 := []byte("some text")

			cryptor = makeCryptor("unknown")
			encoder = format.NewEncoder(cryptor)

			encoded1, err := encoder.Encode(format.BASE64_ENCRYPTED, value1)
			Expect(err).NotTo(HaveOccurred())

			_, err = storeClient.Set(fmt.Sprintf("%s/my/key-1", etcd.V1SchemaRoot), encoded1, etcd.NO_TTL)
			Expect(err).NotTo(HaveOccurred())

			cryptor = makeCryptor("new", "old")

			etcdDB = etcd.NewETCD(format.ENCRYPTED_PROTO, 100, 100, DesiredLRPCreationTimeout, cryptor, storeClient, clock, fakeMetronClient)
			err = etcdDB.PerformEncryption(logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
