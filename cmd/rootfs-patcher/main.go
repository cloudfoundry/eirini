package main

import (
	"flag"
	"os"

	"code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/lager"
)

func main() {
	rootfsVersion := flag.String("rootfs-version", "", "Version of rootfs")
	namespace := flag.String("namespace", "", "Namespace where eirini runs apps")
	timeout := flag.Duration("timeout", -1, "Timeout for waiting for rootfs patching to be finished")
	kubeConfigPath := flag.String("kubeconfig", "", "Config for kubernetes, leave empty to use in cluster config")

	flag.Parse()

	if *rootfsVersion == "" || *namespace == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	kubeClient := cmd.CreateKubeClient(*kubeConfigPath)
	statefulSetClient := kubeClient.AppsV1beta2().StatefulSets(*namespace)
	podClient := kubeClient.CoreV1().Pods(*namespace)

	logger := lager.NewLogger("Pod Patcher")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))
	patcher := rootfspatcher.StatefulSetPatcher{
		Version: *rootfsVersion,
		Client:  statefulSetClient,
		Logger:  logger,
	}

	logger = lager.NewLogger("Pod Waiter")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

	waiter := rootfspatcher.PodWaiter{
		Client:        podClient,
		Logger:        logger,
		RootfsVersion: *rootfsVersion,
		Timeout:       *timeout,
	}

	err := rootfspatcher.PatchAndWait(patcher, waiter)
	cmd.ExitWithError(err)
}
