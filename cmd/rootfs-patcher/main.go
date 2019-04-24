package main

import (
	"flag"
	"os"
	"time"

	"code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/rootfspatcher"
)

func main() {
	rootfsVersion := flag.String("rootfs-version", "", "Version of rootfs")
	namespace := flag.String("namespace", "", "Namespace where eirini runs apps")
	timeout := flag.Duration("timeout", 1*time.Hour, "Timeout for waiting for rootfs patching to be finished")
	kubeConfigPath := flag.String("kubeconfig", "", "Config for kubernetes, leave empty to use in cluster config")

	flag.Parse()

	if *rootfsVersion == "" || *namespace == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	kubeClient := cmd.CreateKubeClient(*kubeConfigPath)
	statefulSetClient := kubeClient.AppsV1beta2().StatefulSets(*namespace)
	podClient := kubeClient.CoreV1().Pods(*namespace)

	patcher := rootfspatcher.StatefulSetPatcher{
		Version: *rootfsVersion,
		Client:  statefulSetClient,
	}

	waiter := rootfspatcher.PodWaiter{
		Client:        podClient,
		Timeout:       *timeout,
		RootfsVersion: *rootfsVersion,
	}

	err := rootfspatcher.PatchAndWait(patcher, waiter)
	cmd.ExitWithError(err)
}
