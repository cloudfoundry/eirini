package models

import (
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"gopkg.in/yaml.v2"
)

type exportCredential struct {
	Name  string
	Type  string
	Value interface{}
}

type exportCredentials struct {
	Credentials []exportCredential
}

type CredentialBulkExport struct {
	Bytes []byte
}

func ExportCredentials(credentials []credentials.Credential) (*CredentialBulkExport, error) {
	exportCreds := exportCredentials{make([]exportCredential, len(credentials))}

	for i, credential := range credentials {
		exportCreds.Credentials[i] = exportCredential{credential.Name, credential.Type, credential.Value}
	}

	result, err := yaml.Marshal(exportCreds)

	if err != nil {
		return nil, err
	}

	return &CredentialBulkExport{result}, nil
}

func (credentialBulkExport *CredentialBulkExport) String() string {
	return string(credentialBulkExport.Bytes)
}
