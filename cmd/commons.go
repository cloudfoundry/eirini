package cmd

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"
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
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}
