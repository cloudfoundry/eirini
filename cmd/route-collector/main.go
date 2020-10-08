package main

import (
	"os"
	"time"

	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
)

const tickerPeriod uint = 30

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running route-collector"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := route.ReadConfig(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	if cfg.EmitPeriodInSeconds == 0 {
		cfg.EmitPeriodInSeconds = tickerPeriod
	}

	logger := lager.NewLogger("route-collector")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	routeEmitter, err := route.NewEmitterFromConfig(cfg.NatsIP, cfg.NatsPort, cfg.NatsPassword, logger)
	cmdcommons.ExitIfError(err)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	namespace := ""

	if !cfg.EnableMultiNamespaceSupport {
		if cfg.Namespace == "" {
			cmdcommons.Exitf("must set namespace in config when enableMultiNamespaceSupport is not set")
		}

		namespace = cfg.Namespace
	}

	collector := k8s.NewRouteCollector(clientset, namespace, logger)

	scheduler := route.CollectorScheduler{
		Collector: collector,
		Emitter:   routeEmitter,
		Scheduler: &util.TickerTaskScheduler{
			Ticker: time.NewTicker(time.Duration(cfg.EmitPeriodInSeconds) * time.Second),
			Logger: logger.Session("scheduler"),
		},
	}
	scheduler.Start()
}
