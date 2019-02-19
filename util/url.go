package util

import (
	"fmt"
	"net/url"
)

func GenerateNatsURL(natsPassword, natsIP string) string {
	natsURL := url.URL{
		Scheme: "nats",
		Host:   fmt.Sprintf("%s:4222", natsIP),
		User:   url.UserPassword("nats", natsPassword),
	}
	return natsURL.String()
}
