package shared_test

import (
	"errors"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/shared/sharedfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apply Options", func() {
	var (
		resource  string
		optionOne *sharedfakes.FakeOption
		optionTwo *sharedfakes.FakeOption
	)

	BeforeEach(func() {
		resource = "the-resource"
		optionOne = new(sharedfakes.FakeOption)
		optionTwo = new(sharedfakes.FakeOption)
	})

	It("applies all options", func() {
		Expect(shared.ApplyOpts(resource, optionOne.Spy, optionTwo.Spy)).To(Succeed())

		Expect(optionOne.CallCount()).To(Equal(1))
		optionOneArg := optionOne.ArgsForCall(0)
		Expect(optionOneArg).To(Equal(resource))

		Expect(optionTwo.CallCount()).To(Equal(1))
		optionTwoArg := optionTwo.ArgsForCall(0)
		Expect(optionTwoArg).To(Equal(resource))
	})

	When("applying an option fails", func() {
		BeforeEach(func() {
			optionOne.Returns(errors.New("opts-error"))
		})

		It("returns the error", func() {
			err := shared.ApplyOpts(resource, optionOne.Spy)
			Expect(err).To(MatchError(ContainSubstring("opts-error")))
		})
	})
})
