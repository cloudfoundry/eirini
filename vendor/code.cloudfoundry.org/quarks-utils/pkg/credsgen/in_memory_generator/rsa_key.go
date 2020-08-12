package inmemorygenerator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"code.cloudfoundry.org/quarks-utils/pkg/credsgen"
	"github.com/pkg/errors"
)

// GenerateRSAKey generates an RSA key using go's standard crypto library
func (g InMemoryGenerator) GenerateRSAKey(name string) (credsgen.RSAKey, error) {
	g.log.Debugf("Generating RSA key %s", name)

	// generate private key
	private, err := rsa.GenerateKey(rand.Reader, g.Bits)
	if err != nil {
		return credsgen.RSAKey{}, errors.Wrapf(err, "Generating private key failed for secret name %s", name)
	}
	privateBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	}
	privatePEM := pem.EncodeToMemory(privateBlock)

	// Calculate public key
	public := private.Public().(*rsa.PublicKey)
	publicSerialized, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return credsgen.RSAKey{}, errors.Wrap(err, "generating public key")
	}
	publicBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicSerialized,
	}
	publicPEM := pem.EncodeToMemory(publicBlock)

	key := credsgen.RSAKey{
		PrivateKey: privatePEM,
		PublicKey:  publicPEM,
	}
	return key, nil
}
