package rootfspatcher_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRootfspatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rootfspatcher Suite")
}
