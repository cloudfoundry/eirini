package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/golang/protobuf/jsonpb"
)

func main() {
	rlpAddr := os.Getenv("LOG_STREAM_ADDR")
	if rlpAddr == "" {
		log.Fatal("LOG_STREAM_ADDR is required")
	}

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatalf("TOKEN is required")
	}

	sourceId := os.Getenv("SOURCE_ID")
	if sourceId == "" {
		log.Fatalf("SOURCE_ID is required")
	}

	c := loggregator.NewRLPGatewayClient(
		rlpAddr,
		loggregator.WithRLPGatewayClientLogger(log.New(os.Stderr, "", log.LstdFlags)),
		loggregator.WithRLPGatewayHTTPClient(&tokenAttacher{
			token: token,
		}),
	)

	es := c.Stream(context.Background(), &loggregator_v2.EgressBatchRequest{
		Selectors: []*loggregator_v2.Selector{
			{
				SourceId: sourceId,
				Message: &loggregator_v2.Selector_Log{
					Log: &loggregator_v2.LogSelector{},
				},
			},
		},
	})

	marshaler := jsonpb.Marshaler{}

	for {
		for _, e := range es() {
			if err := marshaler.Marshal(os.Stdout, e); err != nil {
				log.Fatal(err)
			}
		}
	}
}

type tokenAttacher struct {
	token string
}

func (a *tokenAttacher) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", a.token)
	return http.DefaultClient.Do(req)
}
