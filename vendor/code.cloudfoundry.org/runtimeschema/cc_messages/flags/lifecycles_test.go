package flags_test

import (
	"code.cloudfoundry.org/runtimeschema/cc_messages/flags"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lifecycles", func() {
	Describe("Set", func() {
		var lifecycles flags.LifecycleMap
		BeforeEach(func() {
			lifecycles = flags.LifecycleMap{}
		})
		It("adds the mapping", func() {
			err := lifecycles.Set("foo:bar/baz")
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycles["foo"]).To(Equal("bar/baz"))
		})

		It("errors when the value is not of the form 'lifecycle:path'", func() {
			err := lifecycles.Set("blork")
			Expect(err).To(Equal(flags.ErrLifecycleFormatInvalid))
		})

		It("errors when the value has an empty lifecycle", func() {
			err := lifecycles.Set(":mindy")
			Expect(err).To(Equal(flags.ErrLifecycleNameEmpty))
		})

		It("errors when the value is not of the form 'lifecycle:path'", func() {
			err := lifecycles.Set("blork:")
			Expect(err).To(Equal(flags.ErrLifecyclePathEmpty))
		})
	})
})
