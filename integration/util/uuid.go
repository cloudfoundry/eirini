package util

import (
	"github.com/hashicorp/go-uuid"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
)

func GenerateGUID() string {
	guid, err := uuid.GenerateUUID()
	Expect(err).NotTo(HaveOccurred())

	return guid[:30]
}
