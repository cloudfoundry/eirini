package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/bulk"
	"github.com/julz/cube/blobondemand"
	"github.com/julz/cube/k8s"
	"github.com/julz/cube/registry"
	"github.com/julz/cube/sink"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	app := cli.NewApp()
	app.Name = "cube"
	app.Usage = "Cube - the CF experience, on any scheduler"

	app.Commands = append(app.Commands, cli.Command{
		Name:  "registry",
		Usage: "run an OCI registry backed by the CF blob store",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "rootfs",
				Usage: "a tar file containing the rootfs",
			},
		},
		Action: func(c *cli.Context) {
			blobstore := blobondemand.NewInMemoryStore()

			rootfsTar, err := os.Open(c.String("rootfs"))
			if err != nil {
				log.Fatal(err)
			}

			rootfsDigest, rootfsSize, err := blobstore.Put(rootfsTar)
			if err != nil {
				log.Fatal(err)
			}

			log.Fatal(http.ListenAndServe("0.0.0.0:8080", registry.NewHandler(
				registry.BlobRef{
					Digest: rootfsDigest,
					Size:   rootfsSize,
				},
				make(registry.InMemoryDropletStore),
				blobstore,
			)))
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name:  "sync",
		Usage: "sync CC apps to a given backend",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "kubeconfig",
				Usage: "path to kubernetes client config",
				Value: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
			},
			cli.StringFlag{
				Name:  "ccApi",
				Usage: "URL of the cloud controller api server",
			},
			cli.StringFlag{
				Name:  "ccUser",
				Value: "internal_user",
			},
			cli.StringFlag{
				Name: "ccPass",
			},
			cli.StringFlag{
				Name:  "backend",
				Usage: "backend to use, currently only supported backend is k8s",
			},
		},
		Action: func(c *cli.Context) {
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
		},
	})

	app.Run(os.Args)
}
