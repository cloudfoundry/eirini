package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func ParseAppNameAndIndex(podName string) (string, int, error) {
	sl := strings.Split(podName, "-")

	if len(sl) <= 1 {
		return "", 0, fmt.Errorf("could not parse app name from %s", podName)
	}
	appName := strings.Join(sl[:len(sl)-1], "")
	index, err := strconv.Atoi(sl[len(sl)-1])

	if err != nil {
		return "", 0, errors.Wrap(err, "pod name does not contain an index")
	}

	return appName, index, nil
}
