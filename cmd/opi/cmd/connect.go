package cmd

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/lager"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/k8s"
	"github.com/JulzDiverse/cfclient"
	"github.com/spf13/cobra"
)

var (
	eiriniConfig string
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "connects CloudFoundry with Kubernetes",
	Run:   connect,
}

func connect(cmd *cobra.Command, args []string) {
	path, err := cmd.Flags().GetString("config")
	exitWithError(err)

	cfg := setConfigFromFile(path)
	converger := initBifrost(cfg)

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	handler := handler.New(converger, handlerLogger)

	log.Fatal(http.ListenAndServe("0.0.0.0:8085", handler))
}

func initBifrost(cfg *eirini.Config) eirini.Bifrost {
	config, err := clientcmd.BuildConfigFromFlags("", cfg.Properties.KubeConfig)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	cfClientConfig := &cfclient.Config{
		SkipSslValidation: cfg.Properties.SkipSslValidation,
		Username:          cfg.Properties.CfUsername,
		Password:          cfg.Properties.CfPassword,
		ApiAddress:        cfg.Properties.CcApi,
	}

	cfClient, err := cfclient.NewClient(cfClientConfig)
	exitWithError(err)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Properties.InsecureSkipVerify,
		},
	}}

	syncLogger := lager.NewLogger("bifrost")
	syncLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	kubeNamespace := cfg.Properties.KubeNamespace
	kubeEndpoint := cfg.Properties.KubeEndpoint

	ingressManager := k8s.NewIngressManager(clientset, kubeEndpoint)
	desirer := k8s.NewDesirer(clientset, kubeNamespace, ingressManager)

	return &bifrost.Bifrost{
		Converter:   bifrost.ConvertFunc(bifrost.Convert),
		Desirer:     desirer,
		CfClient:    cfClient,
		Client:      client,
		Logger:      syncLogger,
		RegistryUrl: "http://127.0.0.1:8080",        //for internal use
		RegistryIP:  cfg.Properties.ExternalAddress, //for external use (kube)
	}
}

func setConfigFromFile(path string) *eirini.Config {
	fileBytes, err := ioutil.ReadFile(path)
	exitWithError(err)

	var Conf eirini.Config
	err = yaml.Unmarshal(fileBytes, &Conf)
	exitWithError(err)

	return &Conf
}

func initConnect() {
	connectCmd.Flags().StringVarP(&eiriniConfig, "config", "c", "", "Path to the erini config file")
}

func exitWithError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exit: %s", err.Error())
		os.Exit(1)
	}
}
