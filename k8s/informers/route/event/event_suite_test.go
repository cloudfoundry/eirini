package event_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEvent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Route Event Suite")
}

var ctx context.Context

var _ = BeforeEach(func() {
	ctx = context.Background()
})
