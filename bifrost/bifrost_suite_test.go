package bifrost_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBifrost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bifrost Suite")
}

var ctx context.Context

var _ = BeforeEach(func() {
	ctx = context.Background()
})
