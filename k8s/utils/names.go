package utils

import (
	"regexp"
	"strings"
)

func SanitizeName(name string, fallback string) string {
	validNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	sanitizedName := strings.ReplaceAll(strings.ToLower(name), "_", "-")
	if validNameRegex.MatchString(sanitizedName) {
		return truncateString(sanitizedName, 40)
	}
	return truncateString(fallback, 40)
}

func truncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num]
	}
	return str
}
