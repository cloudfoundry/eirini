package cmd

import (
	"fmt"
	"os"

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
	ExitIfError(err)

	metricsClient, err := metricsclientset.NewForConfig(config)
	ExitIfError(err)

	return metricsClient
}

func CreateKubeClient(kubeConfigPath string) kubernetes.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitIfError(err)

	clientset, err := kubernetes.NewForConfig(config)
	ExitIfError(err)

	return clientset
}

func ExitIfError(err error) {
	if err != nil {
		panic(err)
	}
}

func Exitf(messageFormat string, args ...interface{}) {
	panic(fmt.Sprintf(messageFormat, args...))
}

func GetOrDefault(actualValue, defaultValue string) string {
	if actualValue != "" {
		return actualValue
	}

	return defaultValue
}
