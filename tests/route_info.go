package tests

import (
	"encoding/json"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
)

type RouteInfo struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

func MarshalRoutes(routes []RouteInfo) json.RawMessage {
	bytes, err := json.Marshal(routes)
	Expect(err).NotTo(HaveOccurred())

	rawMessage := json.RawMessage{}
	Expect(rawMessage.UnmarshalJSON(bytes)).To(Succeed())

	return rawMessage
}
