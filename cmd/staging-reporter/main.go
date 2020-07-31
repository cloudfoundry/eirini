package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/informers/staging"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running staging-reporter"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	launchStagingReporter(
		clientset,
		cfg.CAPath,
		cfg.EiriniCertPath,
		cfg.EiriniKeyPath,
		cfg.Namespace,
	)
}

func launchStagingReporter(clientset kubernetes.Interface, ca, eiriniCert, eiriniKey, namespace string) {
	httpClient, err := util.CreateTLSHTTPClient(
		[]util.CertPaths{
			{
				Crt: eiriniCert,
				Key: eiriniKey,
				Ca:  ca,
			},
		},
	)
	cmdcommons.ExitIfError(err)

	stagingLogger := lager.NewLogger("staging-informer")
	stagingLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	reporter := staging.FailedStagingReporter{
		Client: httpClient,
		Logger: stagingLogger,
	}
	stagingInformer := staging.NewInformer(clientset, 0, namespace, reporter, make(chan struct{}), stagingLogger)

	stagingInformer.Start()
}

func readConfigFile(path string) (*eirini.StagingReporterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.StagingReporterConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
