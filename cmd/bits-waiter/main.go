package main

import (
	"flag"
	"os"
	"time"

	"code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/util"
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
	deploymentClient := kubeClient.AppsV1().Deployments(*namespace)

	logger := lager.NewLogger("Bits Waiter")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

	bitsIsReady := func() bool {
		return utils.IsReady(deploymentClient, logger, "bits")
	}

	pollUntilTrue := func(stop <-chan interface{}) {
		util.PollUntilTrue(bitsIsReady, 1*time.Second, stop)
	}

	cmd.ExitWithError(util.RunWithTimeout(pollUntilTrue, *timeout))
}
