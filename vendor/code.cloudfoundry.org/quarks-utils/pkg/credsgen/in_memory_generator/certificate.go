package inmemorygenerator

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/quarks-utils/pkg/credsgen"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/pkg/errors"
)

// GenerateCertificate generates a certificate using Cloudflare's TLS toolkit
func (g InMemoryGenerator) GenerateCertificate(name string, request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
	g.log.Debugf("Generating certificate %s", name)
	cfssllog.Level = cfssllog.LevelWarning

	var certificate credsgen.Certificate
	var err error

	if request.IsCA {
		certificate, err = g.generateCACertificate(request)
		if err != nil {
			return credsgen.Certificate{}, errors.Wrap(err, "Generating CA certificate failed.")
		}
	} else {
		certificate, err = g.generateCertificate(request)
		if err != nil {
			return credsgen.Certificate{}, errors.Wrap(err, "Generating certificate failed.")
		}
	}
	return certificate, nil
}

// GenerateCertificateSigningRequest Generates a certificate signing request and private key
func (g InMemoryGenerator) GenerateCertificateSigningRequest(request credsgen.CertificateGenerationRequest) ([]byte, []byte, error) {
	cfssllog.Level = cfssllog.LevelWarning

	var csReq, privateKey []byte

	// Generate certificate request
	certReq := &csr.CertificateRequest{KeyRequest: &csr.KeyRequest{A: g.Algorithm, S: g.Bits}}

	certReq.Hosts = append(certReq.Hosts, request.CommonName)
	certReq.Hosts = append(certReq.Hosts, request.AlternativeNames...)
	certReq.CN = certReq.Hosts[0]

	sslValidator := &csr.Generator{Validator: genkey.Validator}
	csReq, privateKey, err := sslValidator.ProcessRequest(certReq)
	if err != nil {
		return csReq, privateKey, err
	}
	return csReq, privateKey, nil
}

// generateCertificate Generate a local-issued certificate and private key
func (g InMemoryGenerator) generateCertificate(request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
	if !request.CA.IsCA {
		return credsgen.Certificate{}, errors.Errorf("The passed CA is not a CA")
	}

	cert := credsgen.Certificate{
		IsCA: false,
	}

	// Generate certificate
	signingReq, privateKey, err := g.GenerateCertificateSigningRequest(request)
	if err != nil {
		return credsgen.Certificate{}, err
	}
	// Sign certificate
	signingProfile := &config.SigningProfile{
		Usage:        []string{"server auth", "client auth"},
		Expiry:       time.Duration(g.Expiry*24) * time.Hour,
		ExpiryString: fmt.Sprintf("%dh", g.Expiry*24),
	}
	cert.Certificate, err = g.signCertificate(signingReq, signingProfile, request)
	if err != nil {
		return credsgen.Certificate{}, err
	}
	cert.PrivateKey = privateKey

	return cert, nil
}

// generateCACertificate Generate self-signed root CA certificate and private key
func (g InMemoryGenerator) generateCACertificate(request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
	req := &csr.CertificateRequest{
		CA:         &csr.CAConfig{Expiry: fmt.Sprintf("%dh", g.Expiry*24)},
		CN:         request.CommonName,
		KeyRequest: &csr.KeyRequest{A: g.Algorithm, S: g.Bits},
	}
	ca, csr, privateKey, err := initca.New(req)
	if err != nil {
		return credsgen.Certificate{}, err
	}

	cert := credsgen.Certificate{
		IsCA:        true,
		Certificate: ca,
		PrivateKey:  privateKey,
	}
	if request.CA.IsCA {
		signingProfile := &config.SigningProfile{
			Usage:        []string{"cert sign", "crl sign"},
			ExpiryString: "43800h",
			Expiry:       5 * helpers.OneYear,
			CAConstraint: config.CAConstraint{
				IsCA: true,
			},
		}
		cert.Certificate, err = g.signCertificate(csr, signingProfile, request)
		if err != nil {
			return credsgen.Certificate{}, err
		}
	}

	return cert, nil
}

// Given a signing profile, csr  & request with CA, the certificate is signed by the CA.
func (g InMemoryGenerator) signCertificate(csr []byte, signingProfile *config.SigningProfile, request credsgen.CertificateGenerationRequest) ([]byte, error) {

	policy := &config.Signing{
		Profiles: map[string]*config.SigningProfile{},
		Default:  signingProfile,
	}

	// Parse parent CA
	parentCACert, err := helpers.ParseCertificatePEM([]byte(request.CA.Certificate))
	if err != nil {
		return []byte{}, errors.Wrap(err, "Parsing CA PEM failed.")
	}
	parentCAKey, err := helpers.ParsePrivateKeyPEM([]byte(request.CA.PrivateKey))
	if err != nil {
		return []byte{}, errors.Wrap(err, "Parsing CA private key failed.")
	}

	s, err := local.NewSigner(parentCAKey, parentCACert, signer.DefaultSigAlgo(parentCAKey), policy)
	if err != nil {
		return []byte{}, errors.Wrap(err, "Creating signer failed.")
	}
	certificate, err := s.Sign(signer.SignRequest{Request: string(csr)})
	if err != nil {
		return []byte{}, errors.Wrap(err, "Signing certificate failed.")
	}

	return certificate, nil
}
