package wait_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWaiter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wait Suite")
}
