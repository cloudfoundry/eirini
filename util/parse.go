package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func ParseAppIndex(podName string) (int, error) {
	sl := strings.Split(podName, "-")

	if len(sl) <= 1 {
		return 0, fmt.Errorf("could not parse app name from %s", podName)
	}

	index, err := strconv.Atoi(sl[len(sl)-1])

	if err != nil {
		return 0, errors.Wrapf(err, "pod %s name does not contain an index", podName)
	}

	return index, nil
}
