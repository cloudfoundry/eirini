package cmd

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/k8s"
	"github.com/JulzDiverse/cfclient"
	nats "github.com/nats-io/go-nats"
	"github.com/spf13/cobra"

	// https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
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

	launchRouteEmitter(
		cfg.Properties.KubeConfig,
		cfg.Properties.KubeEndpoint,
		cfg.Properties.KubeNamespace,
		cfg.Properties.NatsPassword,
		cfg.Properties.NatsIP,
	)

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
		APIAddress:        cfg.Properties.CcAPI,
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

	ingressManager := k8s.NewIngressManager(clientset)
	deploymentManager := k8s.NewDeploymentManager(clientset)
	serviceManager := k8s.NewServiceManager(clientset)
	desirer := k8s.NewDesirer(clientset, kubeNamespace, ingressManager, deploymentManager, serviceManager)

	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	registryIP := cfg.Properties.ExternalAddress
	converter := bifrost.NewConverter(cfClient, client, convertLogger, registryIP, "http://127.0.0.1:8080")

	return &bifrost.Bifrost{
		Converter: converter,
		Desirer:   desirer,
		Logger:    syncLogger,
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
	connectCmd.Flags().StringP("config", "c", "", "Path to the erini config file")
}

func launchRouteEmitter(kubeConf, kubeEndpoint, namespace, natsPassword, natsIP string) {
	nc, err := nats.Connect(fmt.Sprintf("nats://nats:%s@%s:4222", natsPassword, natsIP))
	exitWithError(err)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConf)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	workChan := make(chan []route.RegistryMessage)

	rc := route.Collector{
		Client:        clientset,
		Work:          workChan,
		Scheduler:     &route.TickerTaskScheduler{time.NewTicker(time.Second * 15)},
		KubeNamespace: namespace,
		KubeEndpoint:  kubeEndpoint,
	}

	re := route.NewEmitter(&route.NATSPublisher{NatsClient: nc}, workChan, &route.SimpleLoopScheduler{})

	go re.Start()
	go rc.Start()
}

func exitWithError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exit: %s", err.Error())
		os.Exit(1)
	}
}
