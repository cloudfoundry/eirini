package cmd

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/handler"
	k8sevent "code.cloudfoundry.org/eirini/k8s/informers/event"
	k8sroute "code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/stager"
	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/lager"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/tps/cc_client"
	"github.com/JulzDiverse/cfclient"
	nats "github.com/nats-io/go-nats"
	"github.com/spf13/cobra"

	// For gcp and oidc authentication
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
	bifrost := initBifrost(cfg)

	launchRouteEmitter(
		cfg.Properties.KubeConfig,
		cfg.Properties.KubeNamespace,
		cfg.Properties.NatsPassword,
		cfg.Properties.NatsIP,
	)

	tlsConfig, err := loggregator.NewIngressTLSConfig(
		cfg.Properties.LoggregatorCAPath,
		cfg.Properties.LoggregatorCertPath,
		cfg.Properties.LoggregatorKeyPath,
	)
	exitWithError(err)

	loggregatorClient, err := loggregator.NewIngressClient(
		tlsConfig,
		loggregator.WithAddr(cfg.Properties.LoggregatorAddress),
	)
	exitWithError(err)
	defer func() {
		if err = loggregatorClient.CloseSend(); err != nil {
			exitWithError(err)
		}
	}()
	launchMetricsEmitter(
		fmt.Sprintf("%s/namespaces/%s/pods", cfg.Properties.MetricsSourceAddress, cfg.Properties.KubeNamespace),
		loggregatorClient,
	)

	launchEventReporter(
		cfg.Properties.CcInternalAPI,
		cfg.Properties.CCCAPath,
		cfg.Properties.CCCertPath,
		cfg.Properties.CCKeyPath,
		cfg.Properties.KubeConfig,
		cfg.Properties.KubeNamespace,
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
		Namespace:       cfg.Properties.KubeNamespace,
		CCUploaderIP:    cfg.Properties.CcUploaderIP,
		CertsSecretName: cfg.Properties.CCCertsSecretName,
		Client:          clientset,
	}

	stagerCfg := eirini.StagerConfig{
		CfUsername:        cfg.Properties.CfUsername,
		CfPassword:        cfg.Properties.CfPassword,
		APIAddress:        cfg.Properties.CcAPI,
		EiriniAddress:     cfg.Properties.EiriniAddress,
		Image:             getStagerImage(cfg),
		SkipSslValidation: cfg.Properties.SkipSslValidation,
	}

	return stager.New(taskDesirer, stagerCfg)
}

func initBifrost(cfg *eirini.Config) eirini.Bifrost {
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
	desirer := k8s.NewStatefulSetDesirer(clientset, kubeNamespace)

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
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	exitWithError(err)

	var Conf eirini.Config
	err = yaml.Unmarshal(fileBytes, &Conf)
	exitWithError(err)

	return &Conf
}

func initConnect() {
	connectCmd.Flags().StringP("config", "c", "", "Path to the erini config file")
}

func launchRouteEmitter(kubeConf, namespace, natsPassword, natsIP string) {
	nc, err := nats.Connect(fmt.Sprintf("nats://nats:%s@%s:4222", natsPassword, natsIP))
	exitWithError(err)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConf)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	syncPeriod := 10 * time.Second
	workChan := make(chan *route.Message)

	instanceInformer := k8sroute.NewInstanceChangeInformer(clientset, syncPeriod, namespace)
	uriInformer := k8sroute.NewURIChangeInformer(clientset, syncPeriod, namespace)
	re := route.NewEmitter(&route.NATSPublisher{NatsClient: nc}, workChan, &route.SimpleLoopScheduler{})

	go re.Start()
	go instanceInformer.Start(workChan)
	go uriInformer.Start(workChan)
}

func launchMetricsEmitter(source string, loggregatorClient *loggregator.IngressClient) {
	work := make(chan []metrics.Message, 20)

	collector := k8s.NewMetricsCollector(work, &route.SimpleLoopScheduler{}, source)

	forwarder := metrics.NewLoggregatorForwarder(loggregatorClient)
	emitter := metrics.NewEmitter(work, &route.SimpleLoopScheduler{}, forwarder)

	go collector.Start()
	go emitter.Start()
}

func launchEventReporter(uri, ca, cert, key, kubeConf, namespace string) {
	work := make(chan events.CrashReport, 20)
	tlsConf, err := cc_client.NewTLSConfig(cert, key, ca)
	exitWithError(err)

	client := cc_client.NewCcClient(uri, tlsConf)
	reporter := events.NewCrashReporter(work, &route.SimpleLoopScheduler{}, client, lager.NewLogger("instance-crash-reporter"))

	config, err := clientcmd.BuildConfigFromFlags("", kubeConf)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	crashInformer := k8sevent.NewCrashInformer(
		clientset,
		0,
		namespace,
		work,
		make(chan struct{}),
	)

	go crashInformer.Start()
	go crashInformer.Work()
	go reporter.Run()
}

func getStagerImage(cfg *eirini.Config) string {
	if len(cfg.Properties.StagerImageTag) != 0 {
		return fmt.Sprintf("%s:%s", stager.Image, cfg.Properties.StagerImageTag)
	}

	return fmt.Sprintf("%s:%s", stager.Image, stager.DefaultTag)
}

func exitWithError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exit: %s", err.Error())
		os.Exit(1)
	}
}
