package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/nats-io/nats.go"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running route-collector"`
}

func main() {
	var opts options
	_, _ = flags.ParseArgs(&opts, os.Args)
	cfg := readConfig(opts.ConfigFile)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)
	routeEmitter := createRouteEmitter(cfg.NatsIP, cfg.NatsPort, cfg.NatsPassword)

	launchRouteCollector(clientset, cfg.Namespace, routeEmitter)
}

func createRouteEmitter(natsIP string, natsPort int, natsPassword string) route.Emitter {
	nc, err := nats.Connect(util.GenerateNatsURL(natsPassword, natsIP, natsPort), nats.MaxReconnects(-1))
	cmdcommons.ExitWithError(err)

	logger := lager.NewLogger("route")
	logger.RegisterSink(lager.NewPrettySink(os.Stderr, lager.DEBUG))
	emitterLogger := logger.Session("emitter")

	return route.NewMessageEmitter(&route.NATSPublisher{NatsClient: nc}, emitterLogger)
}

func launchRouteCollector(clientset kubernetes.Interface, namespace string, routeEmitter route.Emitter) {
	logger := lager.NewLogger("route-collector")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	collector := k8s.NewRouteCollector(clientset, namespace, logger)
	scheduler := route.CollectorScheduler{
		Collector: collector,
		Emitter:   routeEmitter,
		Scheduler: &util.TickerTaskScheduler{
			Ticker: time.NewTicker(30 * time.Second),
			Logger: logger.Session("scheduler"),
		},
	}

	scheduler.Start()
}

func readConfig(path string) *eirini.RouteEmitterConfig {
	cfg := readRouteEmitterConfigFromFile(path)
	envNATSPassword := os.Getenv("NATS_PASSWORD")
	if envNATSPassword != "" {
		cfg.NatsPassword = envNATSPassword
	}
	return cfg
}

func readRouteEmitterConfigFromFile(path string) *eirini.RouteEmitterConfig {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	cmdcommons.ExitWithError(err)

	var conf eirini.RouteEmitterConfig
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitWithError(err)

	return &conf
}
