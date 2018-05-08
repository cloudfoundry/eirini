package commands

import (
	"strings"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/generate"
	"github.com/cloudfoundry-incubator/credhub-cli/errors"
	"github.com/cloudfoundry-incubator/credhub-cli/models"
)

type GenerateCommand struct {
	CredentialIdentifier string   `short:"n" required:"yes" long:"name" description:"Name of the credential to generate"`
	CredentialType       string   `short:"t" long:"type" description:"Sets the credential type to generate. Valid types include 'password', 'user', 'certificate', 'ssh' and 'rsa'."`
	NoOverwrite          bool     `short:"O" long:"no-overwrite" description:"Credential is not modified if stored value already exists"`
	OutputJSON           bool     `short:"j" long:"output-json" description:"Return response in JSON format"`
	Username             string   `short:"z" long:"username" description:"[User] Sets the username value of the credential"`
	Length               int      `short:"l" long:"length" description:"[Password, User] Length of the generated value (Default: 30)"`
	IncludeSpecial       bool     `short:"S" long:"include-special" description:"[Password, User] Include special characters in the generated value"`
	ExcludeNumber        bool     `short:"N" long:"exclude-number" description:"[Password, User] Exclude number characters from the generated value"`
	ExcludeUpper         bool     `short:"U" long:"exclude-upper" description:"[Password, User] Exclude upper alpha characters from the generated value"`
	ExcludeLower         bool     `short:"L" long:"exclude-lower" description:"[Password, User] Exclude lower alpha characters from the generated value"`
	SSHComment           string   `short:"m" long:"ssh-comment" description:"[SSH] Comment appended to public key to help identify in environment"`
	KeyLength            int      `short:"k" long:"key-length" description:"[Certificate, SSH, RSA] Bit length of the generated key (Default: 2048)"`
	Duration             int      `short:"d" long:"duration" description:"[Certificate] Valid duration (in days) of the generated certificate (Default: 365)"`
	CommonName           string   `short:"c" long:"common-name" description:"[Certificate] Common name of the generated certificate"`
	Organization         string   `short:"o" long:"organization" description:"[Certificate] Organization of the generated certificate"`
	OrganizationUnit     string   `short:"u" long:"organization-unit" description:"[Certificate] Organization unit of the generated certificate"`
	Locality             string   `short:"i" long:"locality" description:"[Certificate] Locality/city of the generated certificate"`
	State                string   `short:"s" long:"state" description:"[Certificate] State/province of the generated certificate"`
	Country              string   `short:"y" long:"country" description:"[Certificate] Country of the generated certificate"`
	AlternativeName      []string `short:"a" long:"alternative-name" description:"[Certificate] A subject alternative name of the generated certificate (may be specified multiple times)"`
	KeyUsage             []string `short:"g" long:"key-usage" description:"[Certificate] Key Usage extensions for the generated certificate (may be specified multiple times)"`
	ExtendedKeyUsage     []string `short:"e" long:"ext-key-usage" description:"[Certificate] Extended Key Usage extensions for the generated certificate (may be specified multiple times)"`
	Ca                   string   `long:"ca" description:"[Certificate] Name of CA used to sign the generated certificate"`
	IsCA                 bool     `long:"is-ca" description:"[Certificate] The generated certificate is a certificate authority"`
	SelfSign             bool     `long:"self-sign" description:"[Certificate] The generated certificate will be self-signed"`
	ClientCommand
}

func (c GenerateCommand) Execute([]string) error {
	if c.CredentialType == "" {
		return errors.NewGenerateEmptyTypeError()
	}

	var parameters interface{}

	c.CredentialType = strings.ToLower(c.CredentialType)

	if c.CredentialType != "user" && len(c.Username) > 0 {
		return errors.NewUserNameOnlyValidForUserType()
	}

	if len(c.Username) > 0 {
		parameters = generate.User{
			Username:       c.Username,
			Length:         c.Length,
			IncludeSpecial: c.IncludeSpecial,
			ExcludeNumber:  c.ExcludeNumber,
			ExcludeUpper:   c.ExcludeUpper,
			ExcludeLower:   c.ExcludeLower,
		}
	} else {
		parameters = models.GenerationParameters{
			IncludeSpecial:   c.IncludeSpecial,
			ExcludeNumber:    c.ExcludeNumber,
			ExcludeUpper:     c.ExcludeUpper,
			ExcludeLower:     c.ExcludeLower,
			Length:           c.Length,
			CommonName:       c.CommonName,
			Organization:     c.Organization,
			OrganizationUnit: c.OrganizationUnit,
			Locality:         c.Locality,
			State:            c.State,
			Country:          c.Country,
			AlternativeName:  c.AlternativeName,
			ExtendedKeyUsage: c.ExtendedKeyUsage,
			KeyUsage:         c.KeyUsage,
			KeyLength:        c.KeyLength,
			Duration:         c.Duration,
			Ca:               c.Ca,
			SelfSign:         c.SelfSign,
			IsCA:             c.IsCA,
			SSHComment:       c.SSHComment,
			Username:         c.Username,
		}
	}

	mode := credhub.Overwrite

	if c.NoOverwrite {
		mode = credhub.NoOverwrite
	}

	credential, err := c.client.GenerateCredential(c.CredentialIdentifier, c.CredentialType, parameters, mode)

	if err != nil {
		return err
	}

	printCredential(c.OutputJSON, credential)

	return nil
}
