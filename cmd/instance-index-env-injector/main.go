package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/webhook"
	eirinix "code.cloudfoundry.org/eirinix"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type options struct {
	ConfigFile   string `short:"c" long:"config" description:"Config for running event-reporter"`
	RegisterOnly bool   `short:"r" long:"register-only" description:"Register mutating webhook and exit"`
	ExecuteOnly  bool   `short:"x" long:"execute-only" description:"Run webservice without registration"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	log := lager.NewLogger("instance-index-env-injector")
	log.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	filterEiriniApps := true

	register := true
	if opts.ExecuteOnly {
		register = false
	}

	managerOptions := eirinix.ManagerOptions{
		Port:                cfg.ServicePort,
		Host:                "0.0.0.0",
		ServiceName:         cfg.ServiceName,
		WebhookNamespace:    cfg.ServiceNamespace,
		FilterEiriniApps:    &filterEiriniApps,
		RegisterWebHook:     &register,
		OperatorFingerprint: cfg.EiriniXOperatorFingerprint,
		KubeConfig:          cfg.ConfigPath,
		Namespace:           cfg.WorkloadsNamespace,
	}

	manager := eirinix.NewManager(managerOptions)
	err = manager.AddExtension(webhook.NewInstanceIndexEnvInjector(log))
	cmdcommons.ExitfIfError(err, "failed to add the instance index env injector extension")

	if opts.RegisterOnly {
		err = manager.RegisterExtensions()
		cmdcommons.ExitfIfError(err, "failed to register the instance index env injector extension")

		return
	}

	log.Fatal("instance-index-env-injector-errored", manager.Start())
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
