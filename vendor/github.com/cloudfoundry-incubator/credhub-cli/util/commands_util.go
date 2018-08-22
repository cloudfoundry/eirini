package util

import (
	"io/ioutil"

	"strings"

	"os"

	credhub_errors "code.cloudfoundry.org/credhub-cli/errors"
)

func ReadFileOrStringFromField(field string) (string, error) {
	_, err := os.Stat(field)

	if err != nil {
		return strings.Replace(field, "\\n", "\n", -1), nil
	}

	dat, err := ioutil.ReadFile(field)
	if err != nil {
		return "", credhub_errors.NewFileLoadError()
	}
	return string(dat), nil
}

func AddDefaultSchemeIfNecessary(serverUrl string) string {
	if strings.Contains(serverUrl, "://") {
		return serverUrl
	} else {
		return "https://" + serverUrl
	}
}
