package utils

import (
	"fmt"
	"regexp"
	"strings"

	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
)

func SanitizeName(name, fallback string) string {
	return SanitizeNameWithMaxStringLen(name, fallback, 40)
}

func SanitizeNameWithMaxStringLen(name, fallback string, maxStringLen int) string {
	validNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	sanitizedName := strings.ReplaceAll(strings.ToLower(name), "_", "-")

	if validNameRegex.MatchString(sanitizedName) {
		return truncateString(sanitizedName, maxStringLen)
	}

	return truncateString(fallback, maxStringLen)
}

func truncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num]
	}

	return str
}

func GetStatefulsetName(lrp *opi.LRP) (string, error) {
	nameSuffix, err := util.Hash(fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version))
	if err != nil {
		return "", errors.Wrap(err, "failed to generate hash")
	}

	namePrefix := fmt.Sprintf("%s-%s", lrp.AppName, lrp.SpaceName)
	namePrefix = SanitizeName(namePrefix, lrp.GUID)

	return fmt.Sprintf("%s-%s", namePrefix, nameSuffix), nil
}
