package main

import (
	"context"
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/go-loggregator"
)

func main() {
	tlsConfig, err := loggregator.NewIngressTLSConfig(
		os.Getenv("CA_CERT_PATH"),
		os.Getenv("CERT_PATH"),
		os.Getenv("KEY_PATH"),
	)
	if err != nil {
		log.Fatal("Could not create TLS config", err)
	}

	client, err := loggregator.NewIngressClient(
		tlsConfig,
		loggregator.WithAddr("localhost:3458"),
	)

	if err != nil {
		log.Fatal("Could not create client", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = client.EmitEvent(
		ctx,
		"Starting sample V2 Client",
		"This sample V2 client is about to emit 50 log envelopes",
	)
	if err != nil {
		log.Fatalf("Failed to emit event: %s", err)
	}

	for i := 0; i < 50; i++ {
		client.EmitLog("some log goes here",
			loggregator.WithSourceInfo("v2-example-source-id", "platform", "v2-example-source-instance"),
		)

		time.Sleep(10 * time.Millisecond)
	}

	startTime := time.Now()
	for i := 0; i < 5; i++ {
		client.EmitTimer("loop_times", startTime, time.Now())
	}

	client.CloseSend()
}
