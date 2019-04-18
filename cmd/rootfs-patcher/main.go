package main

import (
	"os"
	"time"

	"code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/rootfspatcher"
)

func main() {
	rootfsVersion := os.Args[1]
	namespace := os.Args[2]
	timeout, err := time.ParseDuration(os.Args[3])
	cmd.ExitWithError(err)

	kubeClient := cmd.CreateKubeClient("")
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
	cmd.ExitWithError(err)
}
