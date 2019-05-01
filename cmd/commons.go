package cmd

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	// Kubernetes has a tricky way to add authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func CreateMetricsClient(kubeConfigPath string) metricsclientset.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitWithError(err)

	metricsClient, err := metricsclientset.NewForConfig(config)
	ExitWithError(err)

	return metricsClient
}

func CreateKubeClient(kubeConfigPath string) kubernetes.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	ExitWithError(err)

	return clientset
}

func ExitWithError(err error) {
	if err != nil {
		panic(err)
	}
}
