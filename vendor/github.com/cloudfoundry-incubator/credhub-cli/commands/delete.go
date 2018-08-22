package commands

import (
	"fmt"
)

type DeleteCommand struct {
	CredentialIdentifier string `short:"n" long:"name" required:"yes" description:"Name of the credential to delete"`
	ClientCommand
}

func (c *DeleteCommand) Execute([]string) error {
	if err := c.client.Delete(c.CredentialIdentifier); err != nil {
		return err
	}
	fmt.Println("Credential successfully deleted")
	return nil
}
