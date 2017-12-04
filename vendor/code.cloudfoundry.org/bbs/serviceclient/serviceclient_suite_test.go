package serviceclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestServiceclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Serviceclient Suite")
}
