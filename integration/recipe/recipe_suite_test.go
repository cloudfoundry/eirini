package recipe_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRecipe(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Recipe Suite")
}
