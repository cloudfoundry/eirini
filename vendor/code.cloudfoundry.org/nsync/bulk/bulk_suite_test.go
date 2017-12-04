package bulk_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBulk(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bulk Suite")
}
