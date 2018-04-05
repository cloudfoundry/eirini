package main

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/julz/cube/route"
	nats "github.com/nats-io/go-nats"
	"github.com/urfave/cli"
)

func routeEmitterCmd(c *cli.Context) {
	natsPassword := "gjjqystmkiq89n8vrmve"
	natsUser := "nats"
	natsIp := "10.244.0.129"

	nc, err := nats.Connect(fmt.Sprintf("nats://%s:%s@%s:4222", natsUser, natsPassword, natsIp))
	exitWithError(err)

	config, err := clientcmd.BuildConfigFromFlags("", c.String("kube-config"))
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	workChan := make(chan []route.RegistryMessage)

	rc := route.RouteCollector{Client: clientset, Work: workChan, Host: c.String("host")}
	re := route.RouteEmitter{NatsClient: nc, Work: workChan}

	go re.Start()
	go rc.Start(15)

	select {}
}
