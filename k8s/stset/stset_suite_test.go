package stset_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stset Suite")
}

var ctx context.Context

var _ = BeforeEach(func() {
	ctx = context.Background()
})
