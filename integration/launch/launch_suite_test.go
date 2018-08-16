package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLaunch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Launch Suite")
}
