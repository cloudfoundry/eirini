package util

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/pkg/errors"
)

type CertPaths struct {
	Crt, Key, Ca string
}

func CreateTLSHTTPClient(certPaths []CertPaths) (*http.Client, error) {
	pool := x509.NewCertPool()
	certs := []tls.Certificate{}
	for _, c := range certPaths {
		cert, err := tls.LoadX509KeyPair(c.Crt, c.Key)
		if err != nil {
			return nil, errors.Wrap(err, "could not load cert")
		}
		certs = append(certs, cert)

		cacert, err := ioutil.ReadFile(filepath.Clean(c.Ca))
		if err != nil {
			return nil, err
		}
		if ok := pool.AppendCertsFromPEM(cacert); !ok {
			return nil, errors.New("failed to append cert to cert pool")
		}
	}

	tlsConf := &tls.Config{
		Certificates: certs,
		RootCAs:      pool,
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConf}}, nil
}
