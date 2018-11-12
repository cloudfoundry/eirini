package main

import (
	"context"
	"log"
	"os"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

var allSelectors = []*loggregator_v2.Selector{
	{
		Message: &loggregator_v2.Selector_Log{
			Log: &loggregator_v2.LogSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Counter{
			Counter: &loggregator_v2.CounterSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Gauge{
			Gauge: &loggregator_v2.GaugeSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Timer{
			Timer: &loggregator_v2.TimerSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Event{
			Event: &loggregator_v2.EventSelector{},
		},
	},
}

func main() {
	tlsConfig, err := loggregator.NewEgressTLSConfig(
		os.Getenv("CA_CERT_PATH"),
		os.Getenv("CERT_PATH"),
		os.Getenv("KEY_PATH"),
	)
	if err != nil {
		log.Fatal("Could not create TLS config", err)
	}

	loggr := log.New(os.Stderr, "[", log.LstdFlags)
	streamConnector := loggregator.NewEnvelopeStreamConnector(
		os.Getenv("LOGS_API_ADDR"),
		tlsConfig,
		loggregator.WithEnvelopeStreamLogger(loggr),
	)

	rx := streamConnector.Stream(context.Background(), &loggregator_v2.EgressBatchRequest{
		ShardId:   os.Getenv("SHARD_ID"),
		Selectors: allSelectors,
	})

	for {
		batch := rx()

		for _, e := range batch {
			log.Printf("%+v\n", e)
		}
	}
}
