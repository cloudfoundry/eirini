package namespacers_test

import (
	"code.cloudfoundry.org/eirini/bifrost/namespacers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SingleNamespace", func() {

	var namespacer namespacers.SingleNamespace

	BeforeEach(func() {
		namespacer = namespacers.NewSingleNamespace("default-ns")
	})

	It("returns the default namespace if provided namespace is empty", func() {
		actualNs, err := namespacer.GetNamespace("")
		Expect(err).NotTo(HaveOccurred())
		Expect(actualNs).To(Equal("default-ns"))
	})

	When("the requested namespace is the default namespace", func() {
		It("returns the default namespace", func() {
			actualNs, err := namespacer.GetNamespace("default-ns")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualNs).To(Equal("default-ns"))
		})
	})

	When("the requested namespace is not the default namespace", func() {
		It("fails", func() {
			_, err := namespacer.GetNamespace("foo")
			Expect(err).To(MatchError("namespace \"foo\" is not allowed"))
		})
	})
})
