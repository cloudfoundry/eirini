package credhub

import "fmt"

// Error provides errors for the CredHub client
type Error struct {
	Name        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *Error) Error() string {
	if e.Description == "" {
		return e.Name
	}
	return fmt.Sprintf("%s: %s", e.Name, e.Description)
}
