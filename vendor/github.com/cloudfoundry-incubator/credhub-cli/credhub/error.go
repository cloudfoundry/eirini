package credhub

// Error provides errors for the CredHub client
type Error struct {
	Name        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *Error) Error() string {
	return e.Name
}
