package credentials_test

import (
	"encoding/json"

	"gopkg.in/yaml.v2"

	. "code.cloudfoundry.org/credhub-cli/credhub/credentials"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types", func() {
	Describe("Certificate", func() {
		Specify("when decoding and encoding", func() {
			var cred Certificate

			credJson := `{
	"id": "some-id",
	"name": "/example-certificate",
	"type": "certificate",
	"value": {
		"ca": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
		"certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
		"private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
	},
	"version_created_at": "2017-01-01T04:07:18Z"
}`
			credYaml := `id: some-id
name: /example-certificate
type: certificate
value:
  ca: |-
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  certificate: |-
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  private_key: |-
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
version_created_at: 2017-01-01T04:07:18Z`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-certificate"))
			Expect(cred.Type).To(Equal("certificate"))
			Expect(cred.Value.Ca).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
			Expect(cred.Value.Certificate).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
			Expect(cred.Value.PrivateKey).To(Equal("-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-01T04:07:18Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("User", func() {
		Specify("when decoding and encoding", func() {
			var cred User
			credJson := `{
      "id": "some-id",
      "name": "/example-user",
      "type": "user",
      "value": {
        "username": "some-username",
        "password": "some-password",
        "password_hash": "some-password-hash"
      },
      "version_created_at": "2017-01-05T01:01:01Z"
}`

			credYaml := `
id: some-id
name: "/example-user"
type: user
value:
  username: some-username
  password: some-password
  password_hash: some-password-hash
version_created_at: '2017-01-05T01:01:01Z'`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-user"))
			Expect(cred.Type).To(Equal("user"))
			Expect(cred.Value.Username).To(Equal("some-username"))
			Expect(cred.Value.Password).To(Equal("some-password"))
			Expect(cred.Value.PasswordHash).To(Equal("some-password-hash"))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("Password", func() {
		Specify("when decoding and encoding", func() {
			var cred Password

			credJson := ` {
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    }`

			credYaml := `
id: some-id
name: "/example-password"
type: password
value: some-password
version_created_at: '2017-01-05T01:01:01Z'
`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-password"))
			Expect(cred.Type).To(Equal("password"))
			Expect(cred.Value).To(BeEquivalentTo("some-password"))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("JSON", func() {
		Specify("when decoding and encoding", func() {
			var cred JSON

			credJson := ` {
      "id": "some-id",
      "name": "/example-json",
      "type": "json",
      "value": {
        "key": 123,
        "key_list": [
          "val1",
          "val2"
        ],
        "is_true": true
      },
      "version_created_at": "2017-01-01T04:07:18Z"
    }`

			credYaml := `
id: some-id
name: "/example-json"
type: json
value:
  key: 123
  key_list:
  - val1
  - val2
  is_true: true
version_created_at: '2017-01-01T04:07:18Z'
`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			jsonValueString := `{
        "key": 123,
        "key_list": [
          "val1",
          "val2"
        ],
        "is_true": true
      }`

			var unmarshalled values.JSON
			json.Unmarshal([]byte(jsonValueString), &unmarshalled)

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-json"))
			Expect(cred.Type).To(Equal("json"))
			Expect(cred.Value).To(Equal(unmarshalled))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-01T04:07:18Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("Value", func() {
		Specify("when decoding and encoding", func() {
			var cred Value

			credJson := ` {
      "id": "some-id",
      "name": "/example-value",
      "type": "value",
      "value": "some-value",
      "version_created_at": "2017-01-05T01:01:01Z"
    }`

			credYaml := `
id: some-id
name: "/example-value"
type: value
value: some-value
version_created_at: '2017-01-05T01:01:01Z'
`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-value"))
			Expect(cred.Type).To(Equal("value"))
			Expect(cred.Value).To(BeEquivalentTo("some-value"))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("RSA", func() {
		Specify("when decoding and encoding", func() {
			var cred RSA
			credJson := `{
      "id": "some-id",
      "name": "/example-rsa",
      "type": "rsa",
      "value": {
        "public_key": "some-public-key",
        "private_key": "some-private-key"
      },
      "version_created_at": "2017-01-05T01:01:01Z"
}`

			credYaml := `
id: some-id
name: "/example-rsa"
type: rsa
value:
  public_key: some-public-key
  private_key: some-private-key
version_created_at: '2017-01-05T01:01:01Z'`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-rsa"))
			Expect(cred.Type).To(Equal("rsa"))
			Expect(cred.Value.PublicKey).To(Equal("some-public-key"))
			Expect(cred.Value.PrivateKey).To(Equal("some-private-key"))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("SSH", func() {
		Specify("when decoding and encoding", func() {
			var cred SSH
			credJson := `{
      "id": "some-id",
      "name": "/example-ssh",
      "type": "ssh",
      "value": {
        "public_key": "some-public-key",
        "private_key": "some-private-key",
        "public_key_fingerprint": "some-public-key-fingerprint"
      },
      "version_created_at": "2017-01-01T04:07:18Z"
    }`

			credYaml := `
id: some-id
name: "/example-ssh"
type: ssh
value:
  public_key: some-public-key
  private_key: some-private-key
  public_key_fingerprint: some-public-key-fingerprint
version_created_at: '2017-01-01T04:07:18Z'`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-ssh"))
			Expect(cred.Type).To(Equal("ssh"))
			Expect(cred.Value.PublicKey).To(Equal("some-public-key"))
			Expect(cred.Value.PublicKeyFingerprint).To(Equal("some-public-key-fingerprint"))
			Expect(cred.Value.PrivateKey).To(Equal("some-private-key"))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-01T04:07:18Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})

	Describe("Credential", func() {
		Specify("when decoding and encoding", func() {
			var cred Credential
			credJson := `{
      "id": "some-id",
      "name": "/example-ssh",
      "type": "ssh",
      "value": {
        "public_key": "some-public-key",
        "private_key": "some-private-key",
        "public_key_fingerprint": "some-public-key-fingerprint"
      },
      "version_created_at": "2017-01-01T04:07:18Z"
    }`

			credYaml := `
id: some-id
name: "/example-ssh"
type: ssh
value:
  public_key: some-public-key
  private_key: some-private-key
  public_key_fingerprint: some-public-key-fingerprint
version_created_at: '2017-01-01T04:07:18Z'`

			err := json.Unmarshal([]byte(credJson), &cred)

			Expect(err).To(BeNil())

			jsonValueString := `{
        "public_key": "some-public-key",
        "private_key": "some-private-key",
        "public_key_fingerprint": "some-public-key-fingerprint"
      }`
			var jsonValue map[string]interface{}
			err = json.Unmarshal([]byte(jsonValueString), &jsonValue)
			Expect(err).To(BeNil())

			Expect(cred.Id).To(Equal("some-id"))
			Expect(cred.Name).To(Equal("/example-ssh"))
			Expect(cred.Type).To(Equal("ssh"))
			Expect(cred.Value).To(Equal(jsonValue))
			Expect(cred.VersionCreatedAt).To(Equal("2017-01-01T04:07:18Z"))

			jsonOutput, err := json.Marshal(cred)

			Expect(jsonOutput).To(MatchJSON(credJson))

			yamlOutput, err := yaml.Marshal(cred)

			Expect(yamlOutput).To(MatchYAML(credYaml))
		})
	})
})
