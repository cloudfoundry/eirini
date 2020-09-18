package namespacers_test

import (
	"code.cloudfoundry.org/eirini/bifrost/namespacers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MultiNamespace", func() {

	var namespacer namespacers.MultiNamespace

	BeforeEach(func() {
		namespacer = namespacers.NewMultiNamespace("default-ns")
	})

	It("returns the default namespace if provided namespace is empty", func() {
		actualNs, err := namespacer.GetNamespace("")
		Expect(err).NotTo(HaveOccurred())
		Expect(actualNs).To(Equal("default-ns"))
	})

	It("returns the requested non-empty namespace", func() {
		actualNs, err := namespacer.GetNamespace("my-ns")
		Expect(err).NotTo(HaveOccurred())
		Expect(actualNs).To(Equal("my-ns"))
	})

})
