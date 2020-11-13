package bifrost_test

import (
	"code.cloudfoundry.org/eirini/bifrost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespacer", func() {
	var namespacer bifrost.Namespacer

	BeforeEach(func() {
		namespacer = bifrost.NewNamespacer("default-ns")
	})

	It("returns the default namespace if provided namespace is empty", func() {
		Expect(namespacer.GetNamespace("")).To(Equal("default-ns"))
	})

	It("returns the requested non-empty namespace", func() {
		Expect(namespacer.GetNamespace("my-ns")).To(Equal("my-ns"))
	})
})
