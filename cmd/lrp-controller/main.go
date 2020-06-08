package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/controller"
	"code.cloudfoundry.org/eirini/k8s/informers/lrp"
	lrpclientset "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running lrp-controller"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	eiriniCfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	kubeCfg, err := clientcmd.BuildConfigFromFlags("", eiriniCfg.ConfigPath)
	cmdcommons.ExitIfError(err)

	lrpClient, err := lrpclientset.NewForConfig(kubeCfg)
	cmdcommons.ExitIfError(err)

	launchLrpController(
		lrpClient,
		eiriniCfg.CAPath,
		eiriniCfg.EiriniCertPath,
		eiriniCfg.EiriniKeyPath,
		eiriniCfg.EiriniURI,
	)
}

func launchLrpController(lrpClientset lrpclientset.Interface, ca, eiriniCert, eiriniKey, eiriniURI string) {
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

	logger := lager.NewLogger("lrp-informer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	lrpController := controller.NewRestLrp(httpClient, eiriniURI)
	informer := lrp.NewInformer(logger, lrpClientset, lrpController)
	informer.Start()
}

func readConfigFile(path string) (*eirini.LrpControllerConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.LrpControllerConfig
	err = yaml.Unmarshal(fileBytes, &conf)
	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
