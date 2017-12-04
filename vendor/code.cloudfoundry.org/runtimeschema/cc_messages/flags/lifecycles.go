package flags

import (
	"fmt"
	"strings"
)

var (
	ErrLifecycleFormatInvalid = newLifecycleError("not of the form 'lifecycle-name:path'")
	ErrLifecycleNameEmpty     = newLifecycleError("empty lifecycle name")
	ErrLifecyclePathEmpty     = newLifecycleError("empty path")
)

type lifecycleError struct {
	msg string
}

func newLifecycleError(msg string) error {
	return lifecycleError{msg: msg}
}

func (e lifecycleError) Error() string {
	return "Invalid lifecycle value: " + e.msg
}

type LifecycleMap map[string]string

func (s *LifecycleMap) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *LifecycleMap) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return ErrLifecycleFormatInvalid
	}

	if parts[0] == "" {
		return ErrLifecycleNameEmpty
	}

	if parts[1] == "" {
		return ErrLifecyclePathEmpty
	}

	(*s)[parts[0]] = parts[1]
	return nil
}
