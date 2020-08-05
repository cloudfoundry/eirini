package util

import (
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo"
)

const DefaultApplicationServiceAccount = "eirini"

func GetKubeconfig() string {
	kubeconf := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeconf == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			ginkgo.Fail("INTEGRATION_KUBECONFIG not provided, failed to use default: " + err.Error())
		}

		return filepath.Join(homeDir, ".kube", "config")
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

func GetApplicationServiceAccount() string {
	serviceAccountName := os.Getenv("APPLICATION_SERVICE_ACCOUNT")
	if serviceAccountName != "" {
		return serviceAccountName
	}

	return DefaultApplicationServiceAccount
}

func IsUsingDeployedEirini() bool {
	_, set := os.LookupEnv("USE_DEPLOYED_EIRINI")

	return set
}
