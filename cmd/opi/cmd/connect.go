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
	"code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/lager"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/k8s"
	"github.com/JulzDiverse/cfclient"
	nats "github.com/nats-io/go-nats"
	"github.com/spf13/cobra"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
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

	stager := initStager(cfg)

	workChan := make(chan []*eirini.Routes)
	bifrost := initBifrost(cfg, workChan)

	launchRouteEmitter(
		cfg.Properties.KubeConfig,
		cfg.Properties.KubeEndpoint,
		cfg.Properties.KubeNamespace,
		cfg.Properties.NatsPassword,
		cfg.Properties.NatsIP,
		workChan,
	)

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	handler := handler.New(bifrost, stager, handlerLogger)

	log.Println("opi connected")
	log.Fatal(http.ListenAndServe("0.0.0.0:8085", handler))
}

func initStager(cfg *eirini.Config) eirini.Stager {
	clientset := createKubeClient(cfg)
	taskDesirer := &k8s.TaskDesirer{
		Namespace: cfg.Properties.KubeNamespace,
		Client:    clientset,
	}

	stagerCfg := eirini.StagerConfig{
		CfUsername:        cfg.Properties.CfUsername,
		CfPassword:        cfg.Properties.CfPassword,
		APIAddress:        cfg.Properties.CcAPI,
		EiriniAddress:     cfg.Properties.EiriniAddress,
		SkipSslValidation: cfg.Properties.SkipSslValidation,
	}

	return stager.New(taskDesirer, stagerCfg)
}

func initBifrost(cfg *eirini.Config, workChan chan []*eirini.Routes) eirini.Bifrost {
	cfClientConfig := &cfclient.Config{
		SkipSslValidation: cfg.Properties.SkipSslValidation,
		Username:          cfg.Properties.CfUsername,
		Password:          cfg.Properties.CfPassword,
		ApiAddress:        cfg.Properties.CcAPI,
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

	clientset := createKubeClient(cfg)
	desirer := k8s.NewDesirer(kubeNamespace, clientset, k8s.UseStatefulSets, workChan)

	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	registryIP := cfg.Properties.RegistryAddress
	converter := bifrost.NewConverter(cfClient, client, convertLogger, registryIP, "http://127.0.0.1:8080")

	return &bifrost.Bifrost{
		Converter: converter,
		Desirer:   desirer,
		Logger:    syncLogger,
	}
}

func createKubeClient(cfg *eirini.Config) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", cfg.Properties.KubeConfig)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	return clientset
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

func launchRouteEmitter(kubeConf, kubeEndpoint, namespace, natsPassword, natsIP string, workChan chan []*eirini.Routes) {
	nc, err := nats.Connect(fmt.Sprintf("nats://nats:%s@%s:4222", natsPassword, natsIP))
	exitWithError(err)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConf)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)
	lister := k8s.NewServiceRouteLister(clientset, namespace)

	rc := route.Collector{
		RouteLister: lister,
		Work:        workChan,
		Scheduler:   &route.TickerTaskScheduler{Ticker: time.NewTicker(time.Second * 15)},
	}

	re := route.NewEmitter(&route.NATSPublisher{NatsClient: nc}, workChan, &route.SimpleLoopScheduler{}, kubeEndpoint)

	go re.Start()
	go rc.Start()
}

func exitWithError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exit: %s", err.Error())
		os.Exit(1)
	}
}
