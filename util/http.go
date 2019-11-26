package util

import (
	"net/http"

	"code.cloudfoundry.org/tlsconfig"
)

type CertPaths struct {
	Crt, Key, Ca string
}

func CreateTLSHTTPClient(certPaths []CertPaths) (*http.Client, error) {
	tlsOpts := []tlsconfig.TLSOption{tlsconfig.WithInternalServiceDefaults()}
	tlsClientOpts := []tlsconfig.ClientOption{}

	for _, certPath := range certPaths {
		tlsOpts = append(tlsOpts, tlsconfig.WithIdentityFromFile(certPath.Crt, certPath.Key))
		tlsClientOpts = append(tlsClientOpts, tlsconfig.WithAuthorityFromFile(certPath.Ca))
	}

	tlsConfig, err := tlsconfig.Build(tlsOpts...).Client(tlsClientOpts...)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, nil
}
