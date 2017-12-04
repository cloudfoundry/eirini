package encryption_test

import (
	"code.cloudfoundry.org/bbs/encryption"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encryption Flags", func() {
	var encryptionConfig encryption.EncryptionConfig

	JustBeforeEach(func() {
		encryptionConfig = encryption.EncryptionConfig{
			EncryptionKeys: map[string]string{},
		}
	})

	Describe("Validate", func() {
		It("ensures there's at least one encryption key", func() {
			encryptionConfig.ActiveKeyLabel = "label"

			_, _, err := encryptionConfig.Parse()
			Expect(err).To(HaveOccurred())
		})

		It("parses keys properly", func() {
			encryptionConfig.ActiveKeyLabel = "label"
			encryptionConfig.EncryptionKeys["label"] = "key"

			key, keys, err := encryptionConfig.Parse()
			Expect(err).ToNot(HaveOccurred())

			km, err := encryption.NewKeyManager(key, keys)
			Expect(km.EncryptionKey().Label()).To(Equal("label"))
		})

		It("ensures there's a selected active key", func() {
			encryptionConfig.EncryptionKeys["label"] = "key"
			_, _, err := encryptionConfig.Parse()
			Expect(err).To(MatchError("Must select an active encryption key"))
		})

		It("fails if the active key is not on the list", func() {
			encryptionConfig.ActiveKeyLabel = "other-label"
			_, _, err := encryptionConfig.Parse()
			Expect(err).To(MatchError("Must have at least one encryption key set"))
		})

		It("fails if creating a key fails to parse", func() {
			encryptionConfig.ActiveKeyLabel = "label"
			encryptionConfig.EncryptionKeys["label"] = "key"
			encryptionConfig.EncryptionKeys[""] = "invalid"

			_, _, err := encryptionConfig.Parse()
			Expect(err).To(MatchError("A key label is required"))
		})

		It("returns an active key and all the keys", func() {
			encryptionConfig.ActiveKeyLabel = "label"
			encryptionConfig.EncryptionKeys["label"] = "key"
			encryptionConfig.EncryptionKeys["old-label"] = "old-key"

			activeKey, keys, err := encryptionConfig.Parse()
			keyLabels := make([]string, len(keys))
			for _, key := range keys {
				keyLabels = append(keyLabels, key.Label())
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(activeKey.Label()).To(Equal("label"))
			Expect(keyLabels).To(ContainElement("label"))
			Expect(keyLabels).To(ContainElement("old-label"))
		})
	})
})
