package util

import (
	"github.com/hashicorp/go-uuid"
	. "github.com/onsi/gomega" //nolint:golint,stylecheck
)

func GenerateGUID() string {
	guid, err := uuid.GenerateUUID()
	Expect(err).NotTo(HaveOccurred())
	return guid[:30]
}

