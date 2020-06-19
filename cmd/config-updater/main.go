package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/config"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running config-updater"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	eiriniNamespace := os.Getenv(eirini.EnvEiriniNamespace)
	if eiriniNamespace == "" {
		cmdcommons.Exitf("Missing env var %s", eirini.EnvEiriniNamespace)
	}

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	launchConfigUpdater(eiriniNamespace, clientset)
}

func launchConfigUpdater(namespace string, clientset kubernetes.Interface) {
	logger := lager.NewLogger("config-updater")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	secretReplicator := config.NewSecretReplicator(k8s.NewSecretsClient(clientset))

	configInformer := config.NewInformer(clientset, 0, namespace, secretReplicator, make(chan struct{}), logger)

	configInformer.Start()
}

func readConfigFile(path string) (*eirini.ConfigUpdaterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.ConfigUpdaterConfig
	err = yaml.Unmarshal(fileBytes, &conf)
	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
