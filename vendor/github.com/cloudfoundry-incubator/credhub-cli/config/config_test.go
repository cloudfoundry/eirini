// +build !windows

package config_test

import (
	"io/ioutil"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var cfg config.Config

	BeforeEach(func() {
		cfg = config.Config{
			ApiURL:  "http://api.example.com",
			AuthURL: "http://auth.example.com",
		}
	})

	It("places the config file in .cm in the home directory", func() {
		Expect(config.ConfigPath()).To(HaveSuffix(`/.credhub/config.json`))
	})

	Describe("#UpdateTrustedCAs", func() {
		It("reads multiple certs", func() {
			ca1, err := ioutil.ReadFile("../test/server-tls-ca.pem")
			Expect(err).To(BeNil())
			ca2, err := ioutil.ReadFile("../test/auth-tls-ca.pem")
			Expect(err).To(BeNil())

			err = cfg.UpdateTrustedCAs([]string{"../test/server-tls-ca.pem", "../test/auth-tls-ca.pem"})

			Expect(err).To(BeNil())
			Expect(cfg.CaCerts).To(ConsistOf([]string{string(ca1), string(ca2)}))
		})

		It("overrides previous CAs", func() {
			testCa, err := ioutil.ReadFile("../test/server-tls-ca.pem")
			Expect(err).To(BeNil())

			cfg.CaCerts = []string{"cert1", "cert2"}
			err = cfg.UpdateTrustedCAs([]string{"../test/server-tls-ca.pem"})

			Expect(err).To(BeNil())
			Expect(cfg.CaCerts).To(ConsistOf([]string{string(testCa)}))
		})

		It("handles certificate strings as well as files", func() {
			ca1, err := ioutil.ReadFile("../test/server-tls-ca.pem")
			Expect(err).To(BeNil())
			ca2 := "test-ca-string"

			err = cfg.UpdateTrustedCAs([]string{"../test/server-tls-ca.pem", ca2})

			Expect(err).To(BeNil())
			Expect(cfg.CaCerts).To(ConsistOf([]string{string(ca1), ca2}))
		})

		It("handles new lines in certificate strings", func() {
			caWithNewLines := `-----BEGIN CERTIFICATE-----\nFAKE CERTIFICATE CONTENTS\n-----END CERTIFICATE-----`
			expectedCa := "-----BEGIN CERTIFICATE-----\nFAKE CERTIFICATE CONTENTS\n-----END CERTIFICATE-----"

			err := cfg.UpdateTrustedCAs([]string{caWithNewLines})

			Expect(err).To(BeNil())
			Expect(cfg.CaCerts).To(ConsistOf([]string{expectedCa}))
		})

		It("returns an error if a file can't be read", func() {
			invalidCaFile, err := ioutil.TempFile("", "no-read-access")
			Expect(err).To(BeNil())
			// write-only access
			err = invalidCaFile.Chmod(0222)
			Expect(err).To(BeNil())

			validCaFilePath := "../test/server-tls-ca.pem"
			validCaString := "test-ca-string"
			invalidCaFilePath := invalidCaFile.Name()

			_, err = ioutil.ReadFile(validCaFilePath)
			Expect(err).To(BeNil())
			_, err = ioutil.ReadFile(invalidCaFilePath)
			Expect(err).NotTo(BeNil())

			err = cfg.UpdateTrustedCAs([]string{validCaFilePath, validCaString, invalidCaFilePath})

			Expect(err).NotTo(BeNil())
			Expect(cfg.CaCerts).To(HaveLen(0))
		})
	})
})
