package models_test

import (
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/credhub-cli/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = Describe("ExportCredentials", func() {
	credentials := []credentials.Credential{
		credentials.Credential{
			Metadata: credentials.Metadata{
				Id: "valueID",
				Base: credentials.Base{
					Name:             "valueName",
					VersionCreatedAt: "valueCreatedAt",
				},
				Type: "value",
			},
			Value: "test",
		},
		credentials.Credential{
			Metadata: credentials.Metadata{
				Id: "passwordID",
				Base: credentials.Base{
					Name:             "passwordName",
					VersionCreatedAt: "passwordCreatedAt",
				},
				Type: "password",
			},
			Value: "test",
		},
	}

	It("returns a YAML map with a root credential object", func() {
		exportCreds, err := models.ExportCredentials(credentials)

		var v map[string]interface{}
		var mapOfInterfaces []interface{}

		err = yaml.Unmarshal(exportCreds.Bytes, &v)

		Expect(err).To(BeNil())
		Expect(v["credentials"]).NotTo(BeNil())
		Expect(v["credentials"]).To(BeAssignableToTypeOf(mapOfInterfaces))
	})

	It("lists each credential", func() {
		exportCreds, _ := models.ExportCredentials(credentials)

		var v map[string]interface{}
		_ = yaml.Unmarshal(exportCreds.Bytes, &v)

		exportedCredentials := v["credentials"].([]interface{})

		Expect(exportedCredentials).To(HaveLen(len(credentials)))
	})

	It("includes only a name, type and value in each credential", func() {
		expectedKeys := []string{"name", "type", "value"}
		exportCreds, _ := models.ExportCredentials(credentials)

		var v map[string]interface{}
		_ = yaml.Unmarshal(exportCreds.Bytes, &v)

		exportedCredentials := v["credentials"].([]interface{})

		for _, credential := range exportedCredentials {
			c := credential.(map[interface{}]interface{})

			for k := range c {
				Expect(expectedKeys).To(ContainElement(k))
			}
		}
	})

	It("produces YAML that can be reimported", func() {
		exportCreds, _ := models.ExportCredentials(credentials)
		credImporter := &models.CredentialBulkImport{}

		err := credImporter.ReadBytes(exportCreds.Bytes)

		Expect(err).To(BeNil())
	})
})

var _ = Describe("CredentialBulkExport", func() {
	Describe("String", func() {
		testString := "test"
		subject := models.CredentialBulkExport{[]byte(testString)}

		It("returns a string representation of the Bytes", func() {
			Expect(subject.String()).To(Equal(testString))
		})
	})
})
