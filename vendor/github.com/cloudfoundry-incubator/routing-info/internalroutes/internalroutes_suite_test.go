package internalroutes_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestInternalroutes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internalroutes Suite")
}
