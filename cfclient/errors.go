package cfclient

//go:generate go run gen_error.go

import "fmt"

type CloudFoundryErrors struct {
	Errors []CloudFoundryError `json:"errors"`
}

func (cfErrs CloudFoundryErrors) Error() string {
	err := ""

	for _, cfErr := range cfErrs.Errors {
		err += fmt.Sprintf("%s\n", cfErr)
	}

	return err
}

type CloudFoundryError struct {
	Code        int    `json:"code"`
	ErrorCode   string `json:"error_code"`
	Description string `json:"description"`
}

func (cfErr CloudFoundryError) Error() string {
	return fmt.Sprintf("cfclient: error (%d): %s", cfErr.Code, cfErr.ErrorCode)
}
