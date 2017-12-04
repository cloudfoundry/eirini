package sink_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSink(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sink Suite")
}
