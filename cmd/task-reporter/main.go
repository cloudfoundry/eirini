package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
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

	launchTaskReporter(
		clientset,
		cfg.CAPath,
		cfg.CCCertPath,
		cfg.CCKeyPath,
		cfg.Namespace,
	)
}

func launchTaskReporter(clientset kubernetes.Interface, ca, ccCert, ccKey, namespace string) {
	httpClient, err := util.CreateTLSHTTPClient(
		[]util.CertPaths{
			{
				Crt: ccCert,
				Key: ccKey,
				Ca:  ca,
			},
		},
	)
	cmdcommons.ExitIfError(err)

	taskLogger := lager.NewLogger("task-informer")
	taskLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	reporter := task.StateReporter{
		Client:      httpClient,
		Logger:      taskLogger,
		TaskDeleter: initTaskDeleter(namespace, clientset),
	}
	taskInformer := task.NewInformer(clientset, 0, namespace, reporter, make(chan struct{}), taskLogger)

	taskInformer.Start()
}

func initTaskDeleter(namespace string, clientset kubernetes.Interface) task.Deleter {
	logger := lager.NewLogger("task-deleter")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return &k8s.TaskDesirer{
		Namespace: namespace,
		JobClient: clientset.BatchV1().Jobs(namespace),
		Logger:    logger,
	}
}

func readConfigFile(path string) (*eirini.ReporterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.ReporterConfig
	err = yaml.Unmarshal(fileBytes, &conf)
	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
