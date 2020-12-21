package stset_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stset Suite")
}
