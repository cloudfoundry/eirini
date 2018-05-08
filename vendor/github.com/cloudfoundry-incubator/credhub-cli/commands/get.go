package commands

import (
	"fmt"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/errors"
)

type GetCommand struct {
	Name             string `short:"n" long:"name" description:"Name of the credential to retrieve"`
	ID               string `long:"id" description:"ID of the credential to retrieve"`
	NumberOfVersions int    `long:"versions" description:"Number of versions of the credential to retrieve"`
	OutputJSON       bool   `short:"j" long:"output-json" description:"Return response in JSON format"`
	Key              string `short:"k" long:"key" description:"Return only the specified field of the requested credential"`
	ClientCommand
}

func (c *GetCommand) Execute([]string) error {
	var (
		credential credentials.Credential
		err        error
	)

	var arrayOfCredentials []credentials.Credential

	if c.Name != "" {
		if c.NumberOfVersions != 0 {
			if c.Key != "" {
				return errors.NewGetVersionAndKeyError()
			}
			arrayOfCredentials, err = c.client.GetNVersions(c.Name, c.NumberOfVersions)
		} else {
			credential, err = c.client.GetLatestVersion(c.Name)
		}
	} else if c.ID != "" {
		credential, err = c.client.GetById(c.ID)
	} else {
		return errors.NewMissingGetParametersError()
	}

	if err != nil {
		return err
	}

	if arrayOfCredentials != nil {
		output := map[string][]credentials.Credential{
			"versions": arrayOfCredentials,
		}
		printCredential(c.OutputJSON, output)
	} else {
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
		} else {
			printCredential(c.OutputJSON, credential)
		}
	}

	return nil
}
