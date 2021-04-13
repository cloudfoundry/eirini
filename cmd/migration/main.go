package main

import (
	"context"
	"os"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running a migration"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	var cfg eirini.MigrationConfig
	err = cmdcommons.ReadConfigFile(opts.ConfigFile, &cfg)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	stSetClient := client.NewStatefulSet(clientset, cfg.WorkloadsNamespace)
	jobClient := client.NewJob(clientset, cfg.WorkloadsNamespace)
	pdbClient := client.NewPodDisruptionBudget(clientset)
	secretsClient := client.NewSecret(clientset)
	migrationStepsProvider := migrations.CreateMigrationStepsProvider(stSetClient, pdbClient, secretsClient, cfg.WorkloadsNamespace)
	executor := migrations.NewExecutor(stSetClient, jobClient, migrationStepsProvider)

	logger := lager.NewLogger("migration")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	err = executor.Migrate(context.Background(), logger)
	cmdcommons.ExitfIfError(err, "Migration failed")
}
