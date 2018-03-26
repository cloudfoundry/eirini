package st8ger_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSt8ger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "St8ger Suite")
}
