package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/bulk"
	"github.com/JulzDiverse/cfclient"
	"github.com/cloudfoundry-incubator/eirini"
	"github.com/cloudfoundry-incubator/eirini/k8s"
	"github.com/cloudfoundry-incubator/eirini/sink"
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

	kubeNamespace := conf.Properties.KubeNamespace
	kubeEndpoint := conf.Properties.KubeEndpoint

	ingressManager := k8s.NewIngressManager(clientset, kubeEndpoint)
	desirer := k8s.NewDesirer(clientset, kubeNamespace, ingressManager)

	converger := sink.Converger{
		Converter:   sink.ConvertFunc(sink.Convert),
		Desirer:     desirer,
		CfClient:    cfClient,
		Client:      client,
		Logger:      log,
		RegistryUrl: "http://127.0.0.1:8080",         //for internal use
		RegistryIP:  conf.Properties.ExternalAddress, //for external use (kube)
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
			KubeNamespace:      c.String("namespace"),
			KubeEndpoint:       c.String("kubeEndpoint"),
			RegistryEndpoint:   "http://127.0.0.1:8080",
			Backend:            c.String("backend"),
			CcApi:              c.String("ccApi"),
			CfUsername:         c.String("adminUser"),
			CfPassword:         c.String("adminPass"),
			CcUser:             c.String("ccUser"),
			CcPassword:         c.String("ccPass"),
			ExternalAddress:    c.String("externalCubeAddress"),
			SkipSslValidation:  c.Bool("skipSslValidation"),
			InsecureSkipVerify: true,
		},
	}
}

func getIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("couldn't get IP address")
}
