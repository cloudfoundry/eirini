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

		It("removes extra characters", func() {
			Expect(SanitizeName("1234567890-123456789012345678901234567890123456789123456789123456789000", "guid")).To(HaveLen(40))
		})
	})
})
