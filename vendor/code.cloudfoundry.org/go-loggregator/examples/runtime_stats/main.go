package main

import (
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/runtimeemitter"
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
		loggregator.WithLogger(log.New(os.Stdout, "", log.LstdFlags)),
	)

	if err != nil {
		log.Fatal("Could not create client", err)
	}

	runtimeStats := runtimeemitter.New(
		client,
		runtimeemitter.WithInterval(10*time.Second),
	)

	runtimeStats.Run()
}
