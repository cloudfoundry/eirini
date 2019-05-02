package cmd

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	// Kubernetes has a tricky way to add authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func CreateMetricsClient(kubeConfigPath string) metricsclientset.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitWithError(err)

	metricsClient, err := metricsclientset.NewForConfig(config)
	ExitWithError(err)

	return metricsClient
}

func CreateKubeClient(kubeConfigPath string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	ExitWithError(err)

	return clientset
}

func ExitWithError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
