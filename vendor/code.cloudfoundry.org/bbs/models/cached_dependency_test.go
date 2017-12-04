package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
)

var _ = Describe("CachedDependency", func() {
	Describe("Validate", func() {
		var cachedDep *models.CachedDependency

		Context("when the action has 'from' and 'to' are specified", func() {
			It("is valid", func() {
				cachedDep = &models.CachedDependency{
					From: "web_location",
					To:   "local_location",
				}

				err := cachedDep.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the action also has valid 'checksum_value' and 'checksum_algorith'", func() {
				It("is valid", func() {
					cachedDep = &models.CachedDependency{
						From:              "web_location",
						To:                "local_location",
						ChecksumValue:     "some checksum",
						ChecksumAlgorithm: "md5",
					}

					err := cachedDep.Validate()
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		for _, testCase := range []ValidatorErrorCase{
			{
				"from",
				&models.CachedDependency{
					To: "local_location",
				},
			},
			{
				"to",
				&models.CachedDependency{
					From: "web_location",
				},
			},
			{
				"checksum value",
				&models.CachedDependency{
					From:              "web_location",
					To:                "local_location",
					ChecksumAlgorithm: "md5",
				},
			},
			{
				"checksum algorithm",
				&models.CachedDependency{
					From:          "web_location",
					To:            "local_location",
					ChecksumValue: "some checksum",
				},
			},
			{
				"invalid algorithm",
				&models.CachedDependency{
					From:              "web_location",
					To:                "local_location",
					ChecksumAlgorithm: "invalid",
					ChecksumValue:     "some checksum",
				},
			},
		} {
			testValidatorErrorCase(testCase)
		}
	})
})
