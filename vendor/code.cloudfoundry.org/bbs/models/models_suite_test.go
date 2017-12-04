package models_test

import (
	"testing"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type ValidatorErrorCase struct {
	Message string
	models.Validator
}

func testValidatorErrorCase(testCase ValidatorErrorCase) {
	message := testCase.Message
	model := testCase.Validator

	Context("when invalid", func() {
		It("returns an error indicating '"+message+"'", func() {
			err := model.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(message))
		})
	})
}

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Suite")
}
