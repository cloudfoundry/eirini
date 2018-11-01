package blobondemand_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBlobondemand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blobondemand Suite")
}
