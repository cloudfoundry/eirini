package main

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/eirini/rootfspatcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	rootfsVersion := os.Args[1]
	namespace := os.Args[2]
	timeout, err := time.ParseDuration(os.Args[3])
	exitWithError(err)

	kubeClient := createKubeClient("")
	statefulSetClient := kubeClient.AppsV1beta2().StatefulSets(namespace)
	podClient := kubeClient.CoreV1().Pods(namespace)

	waiter := rootfspatcher.PodWaiter{
		Client:        podClient,
		Timeout:       timeout,
		RootfsVersion: rootfsVersion,
	}

	patcher := rootfspatcher.StatefulSetPatcher{
		Version: rootfsVersion,
		Client:  statefulSetClient,
	}

	err = rootfspatcher.PatchAndWait(patcher, waiter)
	exitWithError(err)
}

func createKubeClient(kubeConfigPath string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	return clientset
}

func exitWithError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}
