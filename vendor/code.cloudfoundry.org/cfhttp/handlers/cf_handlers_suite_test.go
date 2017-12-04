package handlers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCfHttp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CfHttp Handlers Suite")
}
