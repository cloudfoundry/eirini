package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/migrations"
	"k8s.io/client-go/kubernetes"

	// Kubernetes has a tricky way to add authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func CreateMetricsClient(kubeConfigPath string) metricsclientset.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitfIfError(err, "Failed to get kubeconfig")

	metricsClient, err := metricsclientset.NewForConfig(config)
	ExitfIfError(err, "Failed to build metrics client")

	return metricsClient
}

func CreateKubeClient(kubeConfigPath string) kubernetes.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitfIfError(err, "Failed to get kubeconfig")

	clientset, err := kubernetes.NewForConfig(config)
	ExitfIfError(err, "Failed to create k8s client")

	return clientset
}

func ExitIfError(err error) {
	ExitfIfError(err, "an unexpected error occurred")
}

func ExitfIfError(err error, message string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s: %w", message, err))
		os.Exit(1)
	}
}

func Exitf(messageFormat string, args ...interface{}) {
	ExitIfError(fmt.Errorf(messageFormat, args...))
}

func GetOrDefault(actualValue, defaultValue string) string {
	if actualValue != "" {
		return actualValue
	}

	return defaultValue
}

func GetEnvOrDefault(envVar, defaultValue string) string {
	return GetOrDefault(os.Getenv(envVar), defaultValue)
}

func VerifyFileExists(filePath, fileName string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		Exitf("%q file at %q does not exist", fileName, filePath)
	}
}

func GetExistingFile(path, defaultPath, name string) string {
	path = GetOrDefault(path, defaultPath)
	VerifyFileExists(path, name)

	return path
}

func GetExistingEnvFile(envVar, defaultPath, name string) string {
	path := GetEnvOrDefault(envVar, defaultPath)
	VerifyFileExists(path, name)

	return path
}

func GetCertPaths(envVar, defaultPath, name string) (string, string, string) {
	crtDir := GetEnvOrDefault(envVar, defaultPath)

	crtPath := filepath.Join(crtDir, eirini.TLSSecretCert)
	VerifyFileExists(crtPath, fmt.Sprintf("%s Cert", name))

	keyPath := filepath.Join(crtDir, eirini.TLSSecretKey)
	VerifyFileExists(keyPath, fmt.Sprintf("%s Key", name))

	caPath := filepath.Join(crtDir, eirini.TLSSecretCA)
	VerifyFileExists(caPath, fmt.Sprintf("%s CA", name))

	return crtPath, keyPath, caPath
}

func CreateMigrationStepsProvider(stSetClient *client.StatefulSet, workloadsNamespace string) migrations.MigrationProvider {
	migrationSteps := []migrations.MigrationStep{
		migrations.NewAdjustCPURequest(stSetClient),
	}

	return migrations.NewMigrationStepsProvider(migrationSteps)
}
