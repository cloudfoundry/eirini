package recipe_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEirini(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Recipe Suite")
}
