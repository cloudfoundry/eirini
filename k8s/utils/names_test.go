package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/k8s/utils"
)

var _ = Describe("Names", func() {
	Describe("SanitizeName", func() {
		It("should lower case the names", func() {
			Expect(SanitizeName("ALL-CAPS-but-not", "guid")).To(Equal("all-caps-but-not"))
		})

		It("should replace underscores with minus", func() {
			Expect(SanitizeName("under_score", "guid")).To(Equal("under-score"))
		})

		It("should fallback to give fallback string if name contains unsupported chracters", func() {
			Expect(SanitizeName("डोरा-дора-dora", "guid")).To(Equal("guid"))
		})
	})
})
