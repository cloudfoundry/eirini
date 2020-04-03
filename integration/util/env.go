package util

import (
	"os"

	"github.com/onsi/ginkgo"
)

func GetKubeconfig() string {
	kubeconf := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeconf == "" {
		ginkgo.Fail("Integration kubeconfig unot provided. Please export INTEGRATION_KUBECONFIG")
	}
	return kubeconf
}

func GetEiriniDockerHubPassword() string {
	password := os.Getenv("EIRINIUSER_PASSWORD")
	if password == "" {
		ginkgo.Skip("eiriniuser password not provided. Please export EIRINIUSER_PASSWORD")
	}
	return password
}
