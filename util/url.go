package util

import (
	"fmt"
	"net/url"
)

func GenerateNatsURL(natsPassword, natsIP string, natsPort int) string {
	natsURL := url.URL{
		Scheme: "nats",
		Host:   fmt.Sprintf("%s:%d", natsIP, natsPort),
		User:   url.UserPassword("nats", natsPassword),
	}

	return natsURL.String()
}
