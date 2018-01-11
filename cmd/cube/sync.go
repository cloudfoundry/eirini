package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/bulk"
	"github.com/julz/cube/k8s"
	"github.com/julz/cube/sink"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func syncCmd(c *cli.Context) {
	fetcher := &bulk.CCFetcher{
		BaseURI:   c.String("ccApi"),
		BatchSize: 50,
		Username:  c.String("ccUser"),
		Password:  c.String("ccPass"),
	}

	config, err := clientcmd.BuildConfigFromFlags("", c.String("kubeconfig"))
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	converger := sink.Converger{
		Converter: sink.ConvertFunc(sink.Convert),
		Desirer:   &k8s.Desirer{Client: clientset},
	}

	log := lager.NewLogger("sync")
	log.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	cancel := make(chan struct{})
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

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
