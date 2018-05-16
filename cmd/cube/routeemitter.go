package main

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/julz/cube/route"
	nats "github.com/nats-io/go-nats"
	"github.com/urfave/cli"
)

func routeEmitterCmd(c *cli.Context) {
	kubeNamespace := c.String("namespace")
	natsPassword := c.String("natsPass")
	natsIP := c.String("natsIP")
	natsUser := "nats"

	nc, err := nats.Connect(fmt.Sprintf("nats://%s:%s@%s:4222", natsUser, natsPassword, natsIP))
	exitWithError(err)

	config, err := clientcmd.BuildConfigFromFlags("", c.String("kube-config"))
	exitWithError(err)

	clientset, err := kubernetes.NewForConfig(config)
	exitWithError(err)

	workChan := make(chan []route.RegistryMessage)

	rc := route.RouteCollector{
		Client:        clientset,
		Work:          workChan,
		Scheduler:     &route.TickerTaskScheduler{time.NewTicker(time.Second * 15)},
		KubeNamespace: kubeNamespace,
	}

	re := route.NewRouteEmitter(nc, workChan, 15)

	go re.Start()
	go rc.Start()

	select {}
}
