package util

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

func ParseAppIndex(podName string) (int, error) {
	sl := strings.Split(podName, "-")

	if len(sl) <= 1 {
		return 0, fmt.Errorf("could not parse app name from %s", podName)
	}
	index, err := strconv.Atoi(sl[len(sl)-1])

	if err != nil {
		return 0, xerrors.Errorf("pod %s name does not contain an index: %v", podName, err)
	}

	return index, nil
}
