package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	selfIp, err := getIP()
	exitWithError(err)

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
					Name:  "externalCubeAddress",
					Value: fmt.Sprintf("%s:8080", selfIp),
					Usage: "The external cube address which will be used by kubernetes to pull images. <host>:<port>",
				},
				cli.StringFlag{
					Name:  "config",
					Usage: "Path to cube config file",
				},
				cli.StringFlag{
					Name:  "namespace",
					Usage: "name of the kubernetes cluster used for app staging",
				},
				cli.StringFlag{
					Name:  "kubeEndpoint",
					Usage: "endpoint of the kubernetes cluster used for app staging",
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
				cli.StringFlag{
					Name:  "namespace",
					Usage: "name of the kubernetes cluster used for app staging",
				},
			},
			Action: stagingCmd,
		},
		{
			Name:  "route",
			Usage: "emit routes to cc",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "kube-config",
					Value: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
				},
				cli.StringFlag{
					Name:  "namespace",
					Usage: "name of the kubernetes cluster used for app staging",
				},
				cli.StringFlag{
					Name:  "natsPass",
					Usage: "Password for NATS messaging system used to communicate with the gorouter",
				},
				cli.StringFlag{
					Name:  "natsIP",
					Usage: "IP of NATS messaging system used to communicate with the gorouter",
				},
			},
			Action: routeEmitterCmd,
		},
	}

	app.Run(os.Args)
}

func exitWithError(err error) {
	if err != nil {
		panic(err)
	}
}
