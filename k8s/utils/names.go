package utils

import (
	"regexp"
	"strings"
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
