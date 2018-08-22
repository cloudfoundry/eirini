package commands

type BulkRegenerateCommand struct {
	SignedBy   string `required:"yes" long:"signed-by" description:"Selects the credential whose children should recursively be regenerated"`
	OutputJSON bool   `short:"j" long:"output-json" description:"Return response in JSON format"`
	ClientCommand
}

func (c *BulkRegenerateCommand) Execute([]string) error {
	credentials, err := c.client.BulkRegenerate(c.SignedBy)
	if err != nil {
		return err
	}

	printCredential(c.OutputJSON, credentials)

	return nil
}
