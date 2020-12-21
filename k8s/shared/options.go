package shared

import "github.com/pkg/errors"

//counterfeiter:generate . Option

type (
	Option func(resource interface{}) error
)

func ApplyOpts(resource interface{}, opts ...Option) error {
	for _, opt := range opts {
		if err := opt(resource); err != nil {
			return errors.Wrap(err, "failed to apply options")
		}
	}

	return nil
}
