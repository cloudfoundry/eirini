package util

import (
	"os"

	"github.com/onsi/ginkgo"
)

func GetEiriniDockerHubPassword() string {
	password := os.Getenv("EIRINIUSER_PASSWORD")
	if password == "" {
		ginkgo.Skip("eiriniuser password not provided. Please export EIRINIUSER_PASSWORD")
	}
	return password
}
