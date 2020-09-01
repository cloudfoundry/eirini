package tests

import (
	"os"
	"path/filepath"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
)

const DefaultApplicationServiceAccount = "eirini"

func GetKubeconfig() string {
	kubeconfPath := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeconfPath != "" {
		return kubeconfPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		Fail("INTEGRATION_KUBECONFIG not provided, failed to use default: " + err.Error())
	}

	kubeconfPath = filepath.Join(homeDir, ".kube", "config")

	_, err = os.Stat(kubeconfPath)
	if os.IsNotExist(err) {
		return ""
	}

	Expect(err).NotTo(HaveOccurred())

	return kubeconfPath
}

func GetEiriniDockerHubPassword() string {
	password := os.Getenv("EIRINIUSER_PASSWORD")
	if password == "" {
		Skip("eiriniuser password not provided. Please export EIRINIUSER_PASSWORD")
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
