package main

import (
	"os"
	"path/filepath"

	"github.com/urfave/cli"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	app := cli.NewApp()
	app.Name = "cube"
	app.Usage = "Cube - the CF experience, on any scheduler"
	app.Commands = []cli.Command{
		{
			Name:  "registry",
			Usage: "run an OCI registry backed by the CF blob store",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "rootfs",
					Usage: "a tar file containing the rootfs",
				},
			},
			Action: registryCmd,
		},
		{
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
				cli.StringFlag{
					Name:  "adminUser",
					Value: "admin",
				},
				cli.StringFlag{
					Name: "adminPass",
				},
				cli.BoolFlag{
					Name: "skipSslValidation",
				},
				cli.StringFlag{
					Name:  "config",
					Usage: "Path to cube config file",
				},
			},
			Action: syncCmd,
		},
		{
			Name:  "stage",
			Usage: "stage CC apps to given backend",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "kubeconfig",
					Usage: "path to kubernetes client config",
					Value: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
				},
				cli.StringFlag{
					Name:  "cf-username",
					Value: "admin",
				},
				cli.StringFlag{
					Name:  "cf-password",
					Value: "admin",
				},
				cli.StringFlag{
					Name:  "cf-endpoint",
					Value: "https://api.bosh-lite.com",
				},
				cli.StringFlag{
					Name: "cube-address",
				},
				cli.BoolFlag{
					Name: "skipSslValidation",
				},
			},
			Action: stagingCmd,
		},
	}

	app.Run(os.Args)
}

func exitWithError(err error) {
	if err != nil {
		panic(err)
	}
}
