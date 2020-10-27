package utils_test

import (
	. "code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Describe("GetStatefulsetName", func() {
		It("calculates the name of an app's backing statefulset", func() {
			statefulsetName, err := GetStatefulsetName(&opi.LRP{
				LRPIdentifier: opi.LRPIdentifier{
					GUID:    "guid",
					Version: "version",
				},
				AppName:   "app",
				SpaceName: "space",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(statefulsetName).To(Equal("app-space-077dc99e95"))
		})

		When("the prefix is too long", func() {
			It("calculates the name of an app's backing statefulset", func() {
				statefulsetName, err := GetStatefulsetName(&opi.LRP{
					LRPIdentifier: opi.LRPIdentifier{
						GUID:    "guid",
						Version: "version",
					},
					AppName:   "very-long-app-name",
					SpaceName: "space-with-very-very-very-very-very-very-very-very-very-long-name",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(statefulsetName).To(Equal("very-long-app-name-space-with-very-very--077dc99e95"))
			})
		})
	})
})
