package utils

import (
	"regexp"
	"strings"
)

func SanitizeName(name string, fallback string) string {
	validNameRegex := regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$")
	sanitizedName := strings.ReplaceAll(strings.ToLower(name), "_", "-")
	if validNameRegex.MatchString(sanitizedName) {
		return sanitizedName
	}
	return fallback
}
