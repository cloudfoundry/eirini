package cmd

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Kubernetes has a tricky way to add authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

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
