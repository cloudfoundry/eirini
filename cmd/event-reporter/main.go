package main

import (
	"crypto/tls"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/events"
	k8sclient "code.cloudfoundry.org/eirini/k8s/client"
	k8sevent "code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/tps/cc_client"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running event-reporter"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	launchEventReporter(
		clientset,
		cfg.CcInternalAPI,
		cfg.CCTLSDisabled,
		cfg.CCCAPath,
		cfg.CCCertPath,
		cfg.CCKeyPath,
		cfg.Namespace,
	)
}

func launchEventReporter(clientset kubernetes.Interface, uri string, tlsDisabled bool, ca, cert, key, namespace string) {
	tlsConf := &tls.Config{} // nolint:gosec // No need to check for min version as the empty config is only used when tls is disabled

	if !tlsDisabled {
		var err error
		tlsConf, err = cc_client.NewTLSConfig(cert, key, ca)
		cmdcommons.ExitIfError(err)
	}

	client := cc_client.NewCcClient(uri, tlsConf)
	crashReporterLogger := lager.NewLogger("instance-crash-reporter")
	crashReporterLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	emitter := events.NewCcCrashEmitter(crashReporterLogger, client)

	crashLogger := lager.NewLogger("instance-crash-informer")
	crashLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	crashInformer := k8sevent.NewCrashInformer(clientset, 0, namespace, make(chan struct{}), crashLogger, k8sevent.NewDefaultCrashEventGenerator(k8sclient.NewEvent(clientset)), emitter)

	crashInformer.Start()
}

func readConfigFile(path string) (*eirini.EventReporterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.EventReporterConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
