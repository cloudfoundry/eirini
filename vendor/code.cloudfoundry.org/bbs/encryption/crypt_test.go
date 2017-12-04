package encryption_test

import (
	"bytes"
	"crypto/des"
	"crypto/rand"
	"io"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/encryption/encryptionfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crypt", func() {
	var cryptor encryption.Cryptor
	var keyManager encryption.KeyManager
	var prng io.Reader

	BeforeEach(func() {
		key, err := encryption.NewKey("label", "pass phrase")
		Expect(err).NotTo(HaveOccurred())
		keyManager, err = encryption.NewKeyManager(key, nil)
		Expect(err).NotTo(HaveOccurred())
		prng = rand.Reader
	})

	JustBeforeEach(func() {
		cryptor = encryption.NewCryptor(keyManager, prng)
	})

	It("successfully encrypts and decrypts with a key", func() {
		input := []byte("some plaintext data")

		encrypted, err := cryptor.Encrypt(input)
		Expect(err).NotTo(HaveOccurred())
		Expect(encrypted.CipherText).NotTo(HaveLen(0))
		Expect(encrypted.CipherText).NotTo(Equal(input))

		plaintext, err := cryptor.Decrypt(encrypted)
		Expect(err).NotTo(HaveOccurred())
		Expect(plaintext).NotTo(HaveLen(0))
		Expect(plaintext).To(Equal(input))
	})

	It("has the expected nonce length", func() {
		input := []byte("some plaintext data")
		encrypted, err := cryptor.Encrypt(input)
		Expect(err).NotTo(HaveOccurred())

		Expect(encrypted.Nonce).To(HaveLen(encryption.NonceSize))
	})

	Context("when the nonce is incorrect", func() {
		It("fails to decrypt", func() {
			input := []byte("some plaintext data")

			encrypted, err := cryptor.Encrypt(input)
			Expect(err).NotTo(HaveOccurred())
			Expect(encrypted.CipherText).NotTo(HaveLen(0))
			Expect(encrypted.CipherText).NotTo(Equal(input))

			encrypted.Nonce = []byte("123456789012")

			_, err = cryptor.Decrypt(encrypted)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cipher: message authentication failed"))
		})
	})

	Context("when the key is not found", func() {
		It("fails to decrypt", func() {
			encrypted := encryption.Encrypted{
				KeyLabel: "doesnt-exist",
				Nonce:    []byte("123456789012"),
			}

			_, err := cryptor.Decrypt(encrypted)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(`Key with label "doesnt-exist" was not found`))
		})
	})

	Context("when the ciphertext is modified", func() {
		It("fails to decrypt", func() {
			input := []byte("some plaintext data")

			encrypted, err := cryptor.Encrypt(input)
			Expect(err).NotTo(HaveOccurred())
			Expect(encrypted.CipherText).NotTo(HaveLen(0))
			Expect(encrypted.CipherText).NotTo(Equal(input))

			encrypted.CipherText[0] ^= 1

			_, err = cryptor.Decrypt(encrypted)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cipher: message authentication failed"))
		})
	})

	Context("when the random number generator fails", func() {
		BeforeEach(func() {
			prng = bytes.NewBuffer([]byte{})
		})

		It("fails to encrypt", func() {
			input := []byte("some plaintext data")

			_, err := cryptor.Encrypt(input)
			Expect(err).To(MatchError(`Unable to generate random nonce: "EOF"`))
		})
	})

	Context("when the encryption key is invalid", func() {
		var key *encryptionfakes.FakeKey

		BeforeEach(func() {
			desCipher, err := des.NewCipher([]byte("12345678"))
			Expect(err).NotTo(HaveOccurred())

			key = &encryptionfakes.FakeKey{}
			key.BlockReturns(desCipher)
			keyManager, err = encryption.NewKeyManager(key, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			input := []byte("some plaintext data")

			_, err := cryptor.Encrypt(input)
			Expect(err).To(MatchError(HavePrefix("Unable to create GCM-wrapped cipher:")))
		})
	})
})
