package main

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/bulk"
	"github.com/JulzDiverse/cfclient"
	"github.com/julz/cube"
	"github.com/julz/cube/k8s"
	"github.com/julz/cube/sink"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func syncCmd(c *cli.Context) {
	var cfClientConfig *cfclient.Config
	var configPath = c.String("config")
	var conf *cube.SyncConfig

	if configPath == "" {
		conf = setConfigFromCLI(c)
	} else {
		conf = setConfigFromFile(configPath)
	}

	fetcher := &bulk.CCFetcher{
		BaseURI:   conf.Properties.CcApi,
		BatchSize: 50,
		Username:  conf.Properties.CcUser,
		Password:  conf.Properties.CcPassword,
	}

	cfClientConfig = &cfclient.Config{
		SkipSslValidation: conf.Properties.SkipSslValidation,
		Username:          conf.Properties.CfUsername,
		Password:          conf.Properties.CfPassword,
		ApiAddress:        conf.Properties.CcApi,
	}

	config, err := clientcmd.BuildConfigFromFlags("", conf.Properties.KubeConfig)
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	cfClient, err := cfclient.NewClient(cfClientConfig)
	exitWithError(err)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: conf.Properties.InsecureSkipVerify},
	}}

	log := lager.NewLogger("sync")
	log.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	converger := sink.Converger{
		Converter:   sink.ConvertFunc(sink.Convert),
		Desirer:     &k8s.Desirer{Client: clientset},
		CfClient:    cfClient,
		Client:      client,
		Logger:      log,
		RegistryUrl: "http://127.0.0.1:8080",
	}

	cancel := make(chan struct{})

	fetch(fetcher, converger, log, cancel, client)
}

func fetch(fetcher *bulk.CCFetcher, converger sink.Converger, log lager.Logger, cancel chan struct{}, client *http.Client) {
	ticker := time.NewTicker(time.Second * 15)
	for range ticker.C {
		log.Info("tick", nil)

		fingerprints, fpErr := fetcher.FetchFingerprints(log, cancel, client)
		desired, desiredErr := fetcher.FetchDesiredApps(log, cancel, client, fingerprints)

		if err := <-fpErr; err != nil {
			log.Error("fetch-fingerprints-failed", err, lager.Data{"Err": err})
			continue
		}

		select {
		case err := <-desiredErr:
			log.Error("fetch-desired-failed", err)
			continue
		case d := <-desired:
			if err := converger.ConvergeOnce(context.Background(), d); err != nil {
				log.Error("converge-once-failed", err)
			}
		}
	}
}

func setConfigFromFile(file string) *cube.SyncConfig {
	fileBytes, err := ioutil.ReadFile(file)
	exitWithError(err)

	var syncConf cube.SyncConfig
	err = yaml.Unmarshal(fileBytes, &syncConf)
	exitWithError(err)

	return &syncConf
}

func setConfigFromCLI(c *cli.Context) *cube.SyncConfig {
	return &cube.SyncConfig{
		Properties: cube.SyncProperties{
			KubeConfig:         c.String("kubeconfig"),
			RegistryEndpoint:   "http://127.0.0.1:8080",
			Backend:            c.String("backend"),
			CcApi:              c.String("ccApi"),
			CfUsername:         c.String("adminUser"),
			CfPassword:         c.String("adminPass"),
			CcUser:             c.String("ccUser"),
			CcPassword:         c.String("ccPass"),
			SkipSslValidation:  c.Bool("skipSslValidation"),
			InsecureSkipVerify: true,
		},
	}
}
