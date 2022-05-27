package stager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stager Suite")
}
