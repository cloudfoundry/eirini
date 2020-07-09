package util

import (
	"fmt"

	"github.com/hashicorp/go-uuid"
	. "github.com/onsi/gomega" //nolint:golint,stylecheck
)

func GenerateGUID() string {
	guid, err := uuid.GenerateUUID()
	Expect(err).NotTo(HaveOccurred())
	return guid[:30]
}

func Guidify(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, GenerateGUID())
}
