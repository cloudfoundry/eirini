package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running task-reporter"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	launchTaskReporter(clientset, cfg)
}

func launchTaskReporter(clientset kubernetes.Interface, cfg eirini.TaskReporterConfig) {
	httpClient := http.DefaultClient

	if !cfg.CCTLSDisabled {
		var err error
		httpClient, err = util.CreateTLSHTTPClient(
			[]util.CertPaths{
				{
					Crt: cfg.CCCertPath,
					Key: cfg.CCKeyPath,
					Ca:  cfg.CAPath,
				},
			},
		)
		cmdcommons.ExitIfError(err)
	}

	taskLogger := lager.NewLogger("task-informer")
	taskLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	reporter := task.StateReporter{
		Client:      httpClient,
		Logger:      taskLogger,
		TaskDeleter: initTaskDeleter(clientset, cfg.EiriniInstance),
	}
	taskInformer := task.NewInformer(clientset, 0, cfg.Namespace, reporter, make(chan struct{}), taskLogger, cfg.EiriniInstance)

	taskInformer.Start()
}

func initTaskDeleter(clientset kubernetes.Interface, eiriniInstance string) task.Deleter {
	logger := lager.NewLogger("task-deleter")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDeleter(
		logger,
		client.NewJob(clientset),
		client.NewSecret(clientset),
		eiriniInstance,
	)
}

func readConfigFile(path string) (eirini.TaskReporterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return eirini.TaskReporterConfig{}, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.TaskReporterConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return conf, errors.Wrap(err, "failed to unmarshal yaml")
}
