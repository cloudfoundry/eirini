package tests

import (
	"github.com/hashicorp/go-uuid"

	// nolint:golint,stylecheck,revive
	. "github.com/onsi/gomega"
)

func GenerateGUID() string {
	guid, err := uuid.GenerateUUID()
	Expect(err).NotTo(HaveOccurred())

	return guid[:30]
}
