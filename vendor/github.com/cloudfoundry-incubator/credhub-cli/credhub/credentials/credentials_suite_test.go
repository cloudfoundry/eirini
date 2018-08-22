package credentials_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCredentials(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credentials Suite")
}
