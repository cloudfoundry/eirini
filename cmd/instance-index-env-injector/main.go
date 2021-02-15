package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/webhook"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running event-reporter"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", cfg.ConfigPath)
	cmdcommons.ExitfIfError(err, "Failed to build kubeconfig")

	log := lager.NewLogger("instance-index-env-injector")
	log.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	logr := util.NewLagerLogr(log)
	ctrl.SetLogger(logr)
	mgr, err := manager.New(kubeConfig, manager.Options{
		// do not serve prometheus metrics; disabled because port clashes during integration tests
		MetricsBindAddress: "0",
		Scheme:             scheme.Scheme,
		Logger:             logr,
		Port:               int(cfg.Port),
		Host:               "0.0.0.0",
		CertDir:            cfg.CertDir,
	})
	cmdcommons.ExitfIfError(err, "Failed to create k8s controller runtime manager")

	decoder, err := admission.NewDecoder(scheme.Scheme)
	cmdcommons.ExitfIfError(err, "Failed to create admission decoder")

	mgr.GetWebhookServer().Register("/", &admission.Webhook{
		Handler: webhook.NewInstanceIndexEnvInjector(log, decoder),
	})

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitfIfError(err, "Failed to start manager")
}

func readConfigFile(path string) (*eirini.InstanceIndexEnvInjectorConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.InstanceIndexEnvInjectorConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
