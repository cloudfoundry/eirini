package commands

import (
	"fmt"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/credhub-cli/errors"
)

type GetCommand struct {
	Name             string `short:"n" long:"name" description:"Name of the credential to retrieve"`
	ID               string `long:"id" description:"ID of the credential to retrieve"`
	NumberOfVersions int    `long:"versions" description:"Number of versions of the credential to retrieve"`
	OutputJSON       bool   `short:"j" long:"output-json" description:"Return response in JSON format"`
	Quiet            bool   `short:"q" long:"quiet" description:"Return value of credential without metadata"`
	Key              string `short:"k" long:"key" description:"Return only the specified field of the requested credential"`
	ClientCommand
}

func (c *GetCommand) convertToValues(credentials []credentials.Credential) []interface{} {
	values := make([]interface{}, len(credentials))
	for i, credential := range credentials {
		values[i] = credential.Value
	}

	return values
}

func (c *GetCommand) printArrayOfCredentials() error {
	if c.Name == "" {
		return errors.NewMissingGetParametersError()
	}

	if c.Key != "" {
		return errors.NewGetVersionAndKeyError()
	}

	arrayOfCredentials, err := c.client.GetNVersions(c.Name, c.NumberOfVersions)
	if err != nil {
		return err
	}

	if c.Quiet {
		values := c.convertToValues(arrayOfCredentials)
		output := map[string][]interface{} {
			"versions": values,
		}
		printCredential(c.OutputJSON, output)
	} else {
		output := map[string][]credentials.Credential{
			"versions": arrayOfCredentials,
		}
		printCredential(c.OutputJSON, output)
	}

	return nil
}

func (c *GetCommand) printCredential() error {
	var (
		credential credentials.Credential
		err        error
	)

	if c.Name != "" {
		credential, err = c.client.GetLatestVersion(c.Name)
	} else if c.ID != "" {
		credential, err = c.client.GetById(c.ID)
	} else {
		return errors.NewMissingGetParametersError()
	}

	if err != nil {
		return err
	}

	if c.Key != "" {
		cred, ok := credential.Value.(map[string]interface{})
		if !ok {
			return nil
		}

		if cred[c.Key] == nil {
			return nil
		}
		switch cred[c.Key].(type) {
		case string:
			fmt.Println(cred[c.Key])

		default:
			printCredential(c.OutputJSON, cred[c.Key])
		}
	} else if c.Quiet {
		if c.OutputJSON {
			return errors.NewOutputJsonAndQuietError()
		}
		printCredential(c.OutputJSON, credential.Value)
	} else {
		printCredential(c.OutputJSON, credential)
	}

	return nil
}

func (c *GetCommand) Execute([]string) error {
	if c.NumberOfVersions != 0 {
		return c.printArrayOfCredentials()
	}

	return c.printCredential()
}
