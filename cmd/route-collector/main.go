package main

import (
	"os"
	"time"

	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
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
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	cfg, err := route.ReadConfig(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	if cfg.EmitPeriodInSeconds == 0 {
		cfg.EmitPeriodInSeconds = tickerPeriod
	}

	logger := lager.NewLogger("route-collector")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	routeEmitter, err := route.NewEmitterFromConfig(cfg.NatsIP, cfg.NatsPort, cfg.NatsPassword, logger)
	cmdcommons.ExitfIfError(err, "Failed to create route emitter")

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)
	podClient := client.NewPod(clientset, cfg.WorkloadsNamespace)
	statefulSetClient := client.NewStatefulSet(clientset, cfg.WorkloadsNamespace)

	collector := k8s.NewRouteCollector(podClient, statefulSetClient, logger)

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
