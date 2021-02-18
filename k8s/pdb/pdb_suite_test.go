package pdb_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPdb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pdb Suite")
}
