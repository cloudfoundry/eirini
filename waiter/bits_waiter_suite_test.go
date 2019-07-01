package waiter_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBitsWaiter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Waiter Suite")
}
