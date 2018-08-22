// CredHub credential types
package credentials

import (
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
)

// Base fields of a credential
type Base struct {
	Name             string `json:"name" yaml:"name"`
	VersionCreatedAt string `json:"version_created_at" yaml:"version_created_at"`
}

type Metadata struct {
	Id   string `json:"id"`
	Base `yaml:",inline"`
	Type string `json:"type"`
}

// A generic credential
//
// Used when the Type of the credential is not known ahead of time.
//
// Value will be as unmarshalled by https://golang.org/pkg/encoding/json/#Unmarshal
type Credential struct {
	Metadata `yaml:",inline"`
	Value    interface{} `json:"value"`
}

// A Value type credential
type Value struct {
	Metadata `yaml:",inline"`
	Value    values.Value `json:"value"`
}

// A JSON type credential
type JSON struct {
	Metadata `yaml:",inline"`
	Value    values.JSON `json:"value"`
}

// A Password type credential
type Password struct {
	Metadata `yaml:",inline"`
	Value    values.Password `json:"value"`
}

// A User type credential
type User struct {
	Metadata `yaml:",inline"`
	Value    struct {
		values.User  `yaml:",inline"`
		PasswordHash string `json:"password_hash" yaml:"password_hash"`
	} `json:"value"`
}

// A Certificate type credential
type Certificate struct {
	Metadata `yaml:",inline"`
	Value    values.Certificate `json:"value"`
}

// An RSA type credential
type RSA struct {
	Metadata `yaml:",inline"`
	Value    values.RSA `json:"value"`
}

// An SSH type credential
type SSH struct {
	Metadata `yaml:",inline"`
	Value    struct {
		values.SSH           `yaml:",inline"`
		PublicKeyFingerprint string `json:"public_key_fingerprint" yaml:"public_key_fingerprint"`
	} `json:"value"`
}

// Type needed for Bulk Regenerate functionality
type BulkRegenerateResults struct {
	Certificates []string `json:"regenerated_credentials" yaml:"regenerated_credentials"`
}

// Types needed for Find functionality
type FindResults struct {
	Credentials []Base `json:"credentials" yaml:"credentials"`
}

type Paths struct {
	Paths []Path `json:"paths" yaml:"paths"`
}

type Path struct {
	Path string `json:"path" yaml:"path"`
}
