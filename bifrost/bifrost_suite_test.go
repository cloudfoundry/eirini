package bifrost_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBifrost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bifrost Suite")
}
