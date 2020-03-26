package docker

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

func Parse(img string) (string, error) {
	named, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image ref")
	}
	return fmt.Sprintf("//%s", named.String()), nil
}
