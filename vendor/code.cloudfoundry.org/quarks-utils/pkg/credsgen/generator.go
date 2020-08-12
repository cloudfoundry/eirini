package credsgen

const (
	// DefaultPasswordLength represents the default length of a generated password
	// (number of characters)
	DefaultPasswordLength = 64
)

// PasswordGenerationRequest specifies the generation parameters for Passwords
type PasswordGenerationRequest struct {
	Length int
}

// CertificateGenerationRequest specifies the generation parameters for Certificates
type CertificateGenerationRequest struct {
	CommonName       string
	AlternativeNames []string
	IsCA             bool
	CA               Certificate
}

// Certificate holds the information about a certificate
type Certificate struct {
	IsCA        bool
	Certificate []byte
	PrivateKey  []byte
}

// SSHKey represents an SSH key
type SSHKey struct {
	PrivateKey  []byte
	PublicKey   []byte
	Fingerprint string
}

// RSAKey represents an RSA key
type RSAKey struct {
	PrivateKey []byte
	PublicKey  []byte
}

// Generator provides an interface for generating credentials like passwords, certificates or SSH and RSA keys
type Generator interface {
	GeneratePassword(name string, request PasswordGenerationRequest) string
	GenerateCertificate(name string, request CertificateGenerationRequest) (Certificate, error)
	GenerateCertificateSigningRequest(request CertificateGenerationRequest) ([]byte, []byte, error)
	GenerateSSHKey(name string) (SSHKey, error)
	GenerateRSAKey(name string) (RSAKey, error)
}
