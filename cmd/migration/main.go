package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running a migration"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	stSetClient := client.NewStatefulSet(clientset, cfg.WorkloadsNamespace)
	pdbClient := client.NewPodDisruptionBudget(clientset)
	provider := migrations.CreateMigrationStepsProvider(stSetClient, pdbClient, cfg.WorkloadsNamespace)
	executor := migrations.NewExecutor(stSetClient, provider)

	logger := lager.NewLogger("migration")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	err = executor.MigrateStatefulSets(logger)
	cmdcommons.ExitfIfError(err, "Migration failed")
}

func readConfigFile(path string) (eirini.MigrationConfig, error) {
	if path == "" {
		return eirini.MigrationConfig{}, nil
	}

	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return eirini.MigrationConfig{}, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.MigrationConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return conf, errors.Wrap(err, "failed to unmarshal yaml")
}
