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
	kubeConfigPath := flag.String("kubeconfig", "", "Config for kubernetes, leave empty to use in cluster config")

	flag.Parse()

	if *rootfsVersion == "" || *namespace == "" {
		flag.PrintDefaults()
		os.Exit(1) // nolint:gomnd
	}

	kubeClient := cmd.CreateKubeClient(*kubeConfigPath)
	statefulSetClient := kubeClient.AppsV1().StatefulSets(*namespace)

	logger := lager.NewLogger("Pod Patcher")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))
	patcher := rootfspatcher.StatefulSetPatcher{
		Version:      *rootfsVersion,
		StatefulSets: statefulSetClient,
		Logger:       logger,
	}

	cmd.ExitIfError(patcher.Patch())
}
