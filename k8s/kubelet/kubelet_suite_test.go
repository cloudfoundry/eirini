package kubelet_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKubelet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubelet Suite")
}

var ctx context.Context

var _ = BeforeEach(func() {
	ctx = context.Background()
})
