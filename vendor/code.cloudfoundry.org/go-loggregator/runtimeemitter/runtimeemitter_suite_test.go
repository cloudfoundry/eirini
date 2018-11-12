package runtimeemitter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRuntimeemitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runtime Emitter Suite")
}
