package main

import (
	"flag"
	"os"

	"code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/lager"
)

func main() {
	namespace := flag.String("namespace", "", "Namespace where eirini runs apps")
	timeout := flag.Duration("timeout", -1, "Timeout for waiting for rootfs patching to be finished")
	kubeConfigPath := flag.String("kubeconfig", "", "Config for kubernetes, leave empty to use in cluster config")

	flag.Parse()

	if *namespace == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	kubeClient := cmd.CreateKubeClient(*kubeConfigPath)
	podClient := kubeClient.CoreV1().Pods(*namespace)

	logger := lager.NewLogger("Bits Waiter")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

	bitsWaiter := rootfspatcher.PodWaiter{
		Logger:           logger,
		PodLister:        podClient,
		Timeout:          *timeout,
		PodLabelSelector: "name=bits",
	}

	err := bitsWaiter.Wait()
	cmd.ExitWithError(err)
}
