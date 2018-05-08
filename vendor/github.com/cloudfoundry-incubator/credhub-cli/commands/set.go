package commands

import (
	"fmt"

	"bufio"
	"os"
	"strings"

	"encoding/json"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/errors"
	"github.com/cloudfoundry-incubator/credhub-cli/util"
)

type SetCommand struct {
	CredentialIdentifier string `short:"n" required:"yes" long:"name" description:"Name of the credential to set"`
	Type                 string `short:"t" long:"type" description:"Sets the credential type. Valid types include 'value', 'json', 'password', 'user', 'certificate', 'ssh' and 'rsa'."`
	NoOverwrite          bool   `short:"O" long:"no-overwrite" description:"Credential is not modified if stored value already exists"`
	Value                string `short:"v" long:"value" description:"[Value, JSON] Sets the value for the credential"`
	CaName               string `short:"m" long:"ca-name" description:"[Certificate] Sets the root CA to a stored CA credential"`
	Root                 string `short:"r" long:"root" description:"[Certificate] Sets the root CA from file or value"`
	Certificate          string `short:"c" long:"certificate" description:"[Certificate] Sets the certificate from file or value"`
	Private              string `short:"p" long:"private" description:"[Certificate, SSH, RSA] Sets the private key from file or value"`
	Public               string `short:"u" long:"public" description:"[SSH, RSA] Sets the public key from file or value"`
	Username             string `short:"z" long:"username" description:"[User] Sets the username value of the credential"`
	Password             string `short:"w" long:"password" description:"[Password, User] Sets the password value of the credential"`
	OutputJSON           bool   `short:"j" long:"output-json" description:"Return response in JSON format"`
	ClientCommand
}

func (c *SetCommand) Execute([]string) error {
	c.Type = strings.ToLower(c.Type)

	if c.Type == "" {
		return errors.NewSetEmptyTypeError()
	}

	c.setFieldsFromInteractiveUserInput()

	err := c.setFieldsFromFileOrString()
	if err != nil {
		return err
	}

	credential, err := c.setCredential()
	if err != nil {
		return err
	}

	printCredential(c.OutputJSON, credential)

	return nil
}

func (c *SetCommand) setFieldsFromInteractiveUserInput() {
	if c.Value == "" && (c.Type == "value" || c.Type == "json") {
		promptForInput("value: ", &c.Value)
	}

	if c.Password == "" && (c.Type == "password" || c.Type == "user") {
		promptForInput("password: ", &c.Password)
	}
}

func (c *SetCommand) setFieldsFromFileOrString() error {
	var err error

	c.Public, err = util.ReadFileOrStringFromField(c.Public)
	if err != nil {
		return err
	}

	c.Private, err = util.ReadFileOrStringFromField(c.Private)
	if err != nil {
		return err
	}

	c.Root, err = util.ReadFileOrStringFromField(c.Root)
	if err != nil {
		return err
	}

	c.Certificate, err = util.ReadFileOrStringFromField(c.Certificate)

	return err
}

func (c *SetCommand) setCredential() (interface{}, error) {
	mode := credhub.Overwrite

	if c.NoOverwrite {
		mode = credhub.NoOverwrite
	}

	var value interface{}

	switch c.Type {
	case "password":
		value = values.Password(c.Password)
	case "certificate":
		value = values.Certificate{
			Ca:          c.Root,
			Certificate: c.Certificate,
			PrivateKey:  c.Private,
			CaName:      c.CaName,
		}
	case "ssh":
		value = values.SSH{
			PublicKey:  c.Public,
			PrivateKey: c.Private,
		}
	case "rsa":
		value = values.RSA{
			PublicKey:  c.Public,
			PrivateKey: c.Private,
		}
	case "user":
		value = values.User{
			Password: c.Password,
			Username: c.Username,
		}
	case "json":
		value = values.JSON{}
		err := json.Unmarshal([]byte(c.Value), &value)
		if err != nil {
			return nil, err
		}
	default:
		value = values.Value(c.Value)
	}
	return c.client.SetCredential(c.CredentialIdentifier, c.Type, value, mode)
}

func promptForInput(prompt string, value *string) {
	fmt.Printf(prompt)
	reader := bufio.NewReader(os.Stdin)
	val, _ := reader.ReadString('\n')
	*value = string(strings.TrimSpace(val))
}
