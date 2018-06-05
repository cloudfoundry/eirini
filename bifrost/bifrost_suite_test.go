package bifrost_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBifrost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bifrost Suite")
}
