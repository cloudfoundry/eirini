package models_test

import (
	"github.com/cloudfoundry-incubator/credhub-cli/errors"
	"github.com/cloudfoundry-incubator/credhub-cli/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CredentialBulkImport", func() {
	Describe("ReadFile()", func() {
		It("parses YAML", func() {
			var credentialBulkImport models.CredentialBulkImport
			err := credentialBulkImport.ReadFile("../test/test_import_file.yml")

			Expect(err).To(BeNil())
			Expect(len(credentialBulkImport.Credentials)).To(Equal(7))

			expectedPassword := make(map[string]interface{})
			expectedPassword["name"] = "/test/password"
			expectedPassword["type"] = "password"
			expectedPassword["value"] = "test-password-value"
			expectedPassword["overwrite"] = true

			Expect(credentialBulkImport.Credentials[0]).To(Equal(expectedPassword))

			expectedValue := make(map[string]interface{})
			expectedValue["name"] = "/test/value"
			expectedValue["type"] = "value"
			expectedValue["value"] = "test-value"
			expectedValue["overwrite"] = true

			Expect(credentialBulkImport.Credentials[1]).To(Equal(expectedValue))

			expectedCertificate := make(map[string]interface{})
			expectedCertificate["name"] = "/test/certificate"
			expectedCertificate["type"] = "certificate"
			expectedCertificate["value"] = map[string]interface{}{
				"ca":          "ca-certificate",
				"certificate": "certificate",
				"private_key": "private-key",
			}
			expectedCertificate["overwrite"] = true

			Expect(credentialBulkImport.Credentials[2]).To(Equal(expectedCertificate))

			expectedRsa := make(map[string]interface{})
			expectedRsa["name"] = "/test/rsa"
			expectedRsa["type"] = "rsa"
			expectedRsa["value"] = map[string]interface{}{
				"public_key":  "public-key",
				"private_key": "private-key",
			}
			expectedRsa["overwrite"] = true

			Expect(credentialBulkImport.Credentials[3]).To(Equal(expectedRsa))

			expectedSsh := make(map[string]interface{})
			expectedSsh["name"] = "/test/ssh"
			expectedSsh["type"] = "ssh"
			expectedSsh["value"] = map[string]interface{}{
				"public_key":  "ssh-public-key",
				"private_key": "private-key",
			}
			expectedSsh["overwrite"] = true

			Expect(credentialBulkImport.Credentials[4]).To(Equal(expectedSsh))

			expectedUser := make(map[string]interface{})
			expectedUser["name"] = "/test/user"
			expectedUser["type"] = "user"
			expectedUser["value"] = map[string]interface{}{
				"username": "covfefe",
				"password": "test-user-password",
			}
			expectedUser["overwrite"] = true

			Expect(credentialBulkImport.Credentials[5]).To(Equal(expectedUser))

			expectedJson := make(map[string]interface{})
			expectedJson["name"] = "/test/json"
			expectedJson["type"] = "json"
			expectedJson["value"] = map[string]interface{}{
				"arbitrary_object": map[string]interface{}{
					"nested_array": []interface{}{
						"array_val1",
						map[string]interface{}{"array_object_subvalue": "covfefe"},
					},
				},
				"1":    "key is not a string",
				"3.14": "pi",
				"true": "key is a bool",
			}
			expectedJson["overwrite"] = true

			Expect(credentialBulkImport.Credentials[6]).To(Equal(expectedJson))
		})

	})
	Describe("improper formatting", func() {
		Context("when first line is credentials tag", func() {
			credentials := `credentials:
- name: /test/password
  type: password
  value: test-password-value`
			It("does not return an error", func() {
				var credentialBulkImport models.CredentialBulkImport
				error := credentialBulkImport.ReadBytes([]byte(credentials))
				Expect(error).To(BeNil())
			})
		})

		Context("when first line is credentials tag with trailing white space", func() {
			credentials := "credentials:   \n" +
				`- name: /test/password
  type: password
  value: test-password-value`
			It("does not return an error", func() {
				var credentialBulkImport models.CredentialBulkImport
				error := credentialBulkImport.ReadBytes([]byte(credentials))
				Expect(error).To(BeNil())
			})
		})

		Context("when first line is not credentials tag", func() {
			credentials := `not-credentials:
- name: /test/password
  type: password
  value: test-password-value`
			It("does not return an error", func() {
				var credentialBulkImport models.CredentialBulkImport
				error := credentialBulkImport.ReadBytes([]byte(credentials))
				Expect(error).To(Equal(errors.NewNoCredentialsTag()))
			})
		})

		Context("when yaml is incorrect", func() {
			credentials := `credentials:
1
2
			`
			It("does not return an error", func() {
				var credentialBulkImport models.CredentialBulkImport
				error := credentialBulkImport.ReadBytes([]byte(credentials))
				Expect(error).To(Equal(errors.NewInvalidImportYamlError()))
			})
		})
	})
})
