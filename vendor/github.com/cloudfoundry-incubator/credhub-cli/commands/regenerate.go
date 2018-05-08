package commands

type RegenerateCommand struct {
	CredentialIdentifier string `required:"yes" short:"n" long:"name" description:"Selects the credential to regenerate"`
	OutputJSON           bool   `short:"j" long:"output-json" description:"Return response in JSON format"`
	ClientCommand
}

func (c *RegenerateCommand) Execute([]string) error {
	credential, err := c.client.Regenerate(c.CredentialIdentifier)

	if err != nil {
		return err
	}

	printCredential(c.OutputJSON, credential)

	return nil
}
