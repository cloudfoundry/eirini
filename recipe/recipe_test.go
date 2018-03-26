package main_test

import (
	. "github.com/julz/cube/recipe"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recipe", func() {
	Context("Unzip", func() {
		It("should return an error if target or source is empty", func() {
			err := Unzip("", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("source or destination path not defined")))
		})

		It("simply untars a tar file", func() {
			err := Unzip("test/go", "test")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
