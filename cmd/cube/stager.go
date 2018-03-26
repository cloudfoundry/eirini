package main

import (
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/julz/cube"
	"github.com/julz/cube/k8s"
	"github.com/julz/cube/st8ger"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func stagingCmd(c *cli.Context) {
	config, err := clientcmd.BuildConfigFromFlags("", c.String("kubeconfig"))

	exitWithError(err)
	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	taskDesirer := k8s.TaskDesirer{Client: clientset}

	st8 := st8ger.St8ger{
		taskDesirer,
	}

	logger := lager.NewLogger("st8r")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	backendConfig := cube.BackendConfig{
		CfUsername:        c.String("cf-username"),
		CfPassword:        c.String("cf-password"),
		ApiAddress:        c.String("cf-endpoint"),
		CubeAddress:       c.String("cube-address"),
		SkipSslValidation: c.Bool("skipSslValidation"),
	}

	backend := st8ger.NewBackend(backendConfig, logger)

	handler := st8ger.New(st8, backend, logger)

	log.Fatal(http.ListenAndServe("0.0.0.0:8085", handler))
}
