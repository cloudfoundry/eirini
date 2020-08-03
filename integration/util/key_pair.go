package util

import (
	"fmt"

	"code.cloudfoundry.org/tlsconfig/certtest"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
)

func GenerateKeyPair(name string) (string, string) {
	authority, err := certtest.BuildCA(name)
	Expect(err).NotTo(HaveOccurred())
	cert, err := authority.BuildSignedCertificate(name, certtest.WithDomains(name))
	Expect(err).NotTo(HaveOccurred())

	certData, keyData, err := cert.CertificatePEMAndPrivateKey()
	Expect(err).NotTo(HaveOccurred())

	metricsCertPath := WriteTempFile(certData, fmt.Sprintf("%s-cert", name))
	metricsKeyPath := WriteTempFile(keyData, fmt.Sprintf("%s-key", name))

	return metricsCertPath, metricsKeyPath
}
