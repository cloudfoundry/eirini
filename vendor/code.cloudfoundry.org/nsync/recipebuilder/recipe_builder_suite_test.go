package recipebuilder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRecipeBuilder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RecipeBuilder Suite")
}
