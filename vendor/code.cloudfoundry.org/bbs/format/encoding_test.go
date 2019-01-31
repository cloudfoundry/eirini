package format_test

import (
	"encoding/base64"
	"errors"
	"io"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/encryption/encryptionfakes"
	"code.cloudfoundry.org/bbs/format"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encoding", func() {
	var encoder format.Encoder
	var prng io.Reader
	var cryptor encryption.Cryptor

	BeforeEach(func() {
		key, err := encryption.NewKey("label", "some pass phrase")
		Expect(err).NotTo(HaveOccurred())

		keyManager, err := encryption.NewKeyManager(key, nil)
		Expect(err).NotTo(HaveOccurred())

		prng = &zeroReader{}
		cryptor = encryption.NewCryptor(keyManager, prng)
	})

	JustBeforeEach(func() {
		encoder = format.NewEncoder(cryptor)
	})

	Describe("Encode", func() {
		Describe("BASE64_ENCRYPTED", func() {
			It("returns the base64 encoded ciphertext with an encoding type prefix", func() {
				payload := []byte("some-payload")
				encoded, err := encoder.Encode(payload)
				Expect(err).NotTo(HaveOccurred())

				Expect(encoded[0:2]).To(Equal(format.BASE64_ENCRYPTED[:]))
				decoded, err := base64.StdEncoding.DecodeString(string(encoded[2:]))
				Expect(err).NotTo(HaveOccurred())

				labelLength := decoded[0]
				decoded = decoded[1:]

				label := string(decoded[:labelLength])
				decoded = decoded[labelLength:]

				nonce := decoded[:encryption.NonceSize]
				ciphertext := decoded[encryption.NonceSize:]

				Expect(labelLength).To(BeEquivalentTo(len("label")))
				Expect(label).To(Equal("label"))

				encrypted := encryption.Encrypted{
					KeyLabel:   label,
					Nonce:      nonce,
					CipherText: ciphertext,
				}

				decrypted, err := cryptor.Decrypt(encrypted)
				Expect(err).NotTo(HaveOccurred())

				Expect(decrypted).To(Equal(payload))
			})

			Context("when encryption fails", func() {
				var cryptError = errors.New("boom")

				BeforeEach(func() {
					fakeCryptor := &encryptionfakes.FakeCryptor{}
					fakeCryptor.EncryptReturns(encryption.Encrypted{}, cryptError)
					cryptor = fakeCryptor
				})

				It("it returns the error", func() {
					payload := []byte("some-payload")
					_, err := encoder.Encode(payload)
					Expect(err).To(MatchError("boom"))
				})
			})
		})
	})

	Describe("Decode", func() {
		Describe("BASE64_ENCRYPTED", func() {
			It("returns the decrypted payload without an encoding type prefix", func() {
				payload := []byte("payload")
				encrypted, err := cryptor.Encrypt(payload)
				Expect(err).NotTo(HaveOccurred())

				encoded := []byte{}
				encoded = append(encoded, byte(len(encrypted.KeyLabel)))
				encoded = append(encoded, []byte(encrypted.KeyLabel)...)
				encoded = append(encoded, encrypted.Nonce...)
				encoded = append(encoded, encrypted.CipherText...)
				encoded = append(format.BASE64_ENCRYPTED[:], []byte(base64.StdEncoding.EncodeToString(encoded))...)

				decoded, err := encoder.Decode(encoded)
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded).To(Equal(payload))
			})
		})

		Describe("unkown encoding", func() {
			It("fails with an unknown encoding error", func() {
				payload := []byte("99some-payload")
				_, err := encoder.Decode(payload)

				Expect(err).To(HaveOccurred())
			})
		})
	})
})

type zeroReader struct{}

func (zr *zeroReader) Read(target []byte) (int, error) {
	for i := 0; i < len(target); i++ {
		target[i] = 0
	}
	return len(target), nil
}
