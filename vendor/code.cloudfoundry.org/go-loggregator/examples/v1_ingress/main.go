package main

import (
	"log"
	"time"

	"github.com/cloudfoundry/dropsonde"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/v1"
)

func main() {
	dropsonde.Initialize("127.0.0.1:3457", "example-source")

	client, err := v1.NewClient()
	if err != nil {
		log.Fatal("Could not create client", err)
	}

	for {
		client.EmitLog("some log goes here",
			loggregator.WithSourceInfo("v1-example-source-id", "platform", "v1-example-source-instance"),
		)
		time.Sleep(time.Second)
	}
}
