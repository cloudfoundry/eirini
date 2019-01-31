package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

// AddTLSParams appends necessary extra parameters to the
// connection string if tls verifications is enabled.  If
// sqlEnableIdentityVerification is true, turn on hostname/identity
// verification, otherwise only ensure that the server certificate is signed by
// one of the CAs in sqlCACertFile.
func AddTLSParams(
	logger lager.Logger,
	driverName,
	databaseConnectionString,
	sqlCACertFile string,
	sqlEnableIdentityVerification bool,
) string {
	switch driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(databaseConnectionString)
		if err != nil {
			logger.Fatal("invalid-db-connection-string", err, lager.Data{"connection-string": databaseConnectionString})
		}

		if sqlCACertFile != "" {
			certBytes, err := ioutil.ReadFile(sqlCACertFile)
			if err != nil {
				logger.Fatal("failed-to-read-sql-ca-file", err)
			}

			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
				logger.Fatal("failed-to-parse-sql-ca", err)
			}

			tlsConfig := generateTLSConfig(logger, caCertPool, sqlEnableIdentityVerification)
			err = mysql.RegisterTLSConfig("bbs-tls", tlsConfig)
			if err != nil {
				logger.Fatal("cannot-register-tls-config", err)
			}
			cfg.TLSConfig = "bbs-tls"
		}
		cfg.Timeout = 10 * time.Minute
		cfg.ReadTimeout = 10 * time.Minute
		cfg.WriteTimeout = 10 * time.Minute
		databaseConnectionString = cfg.FormatDSN()
	case "postgres":
		var err error
		databaseConnectionString, err = pq.ParseURL(databaseConnectionString)
		if err != nil {
			logger.Fatal("invalid-db-connection-string", err, lager.Data{"connection-string": databaseConnectionString})
		}
		if sqlCACertFile == "" {
			databaseConnectionString = databaseConnectionString + " sslmode=disable"
		} else {
			databaseConnectionString = fmt.Sprintf("%s sslmode=verify-ca sslrootcert=%s", databaseConnectionString, sqlCACertFile)
		}
	}

	return databaseConnectionString
}

func generateTLSConfig(logger lager.Logger, caCertPool *x509.CertPool, sqlEnableIdentityVerification bool) *tls.Config {
	var tlsConfig *tls.Config

	if sqlEnableIdentityVerification {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: false,
			RootCAs:            caCertPool,
		}
	} else {
		tlsConfig = &tls.Config{
			InsecureSkipVerify:    true,
			RootCAs:               caCertPool,
			VerifyPeerCertificate: generateCustomTLSVerificationFunction(caCertPool),
		}
	}

	return tlsConfig
}

func generateCustomTLSVerificationFunction(caCertPool *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		opts := x509.VerifyOptions{
			Roots:         caCertPool,
			CurrentTime:   time.Now(),
			DNSName:       "",
			Intermediates: x509.NewCertPool(),
		}

		certs := make([]*x509.Certificate, len(rawCerts))
		for i, rawCert := range rawCerts {
			cert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				return err
			}
			certs[i] = cert
		}

		for i, cert := range certs {
			if i == 0 {
				continue
			}

			opts.Intermediates.AddCert(cert)
		}

		_, err := certs[0].Verify(opts)
		if err != nil {
			return err
		}

		return nil
	}
}
