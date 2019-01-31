package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
)

var _ = Describe("ImageLayer", func() {
	Describe("Validate", func() {
		var layer *models.ImageLayer

		Context("when 'url', 'destination_path', 'media_type' are specified", func() {
			It("is valid", func() {
				layer = &models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					MediaType:       models.MediaTypeTgz,
					LayerType:       models.LayerTypeShared,
				}

				err := layer.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the action also has valid 'digest_value' and 'digest_algorithm'", func() {
				It("is valid", func() {
					layer = &models.ImageLayer{
						Url:             "web_location",
						DestinationPath: "local_location",
						DigestValue:     "some digest",
						DigestAlgorithm: models.DigestAlgorithmSha256,
						MediaType:       models.MediaTypeTgz,
						LayerType:       models.LayerTypeExclusive,
					}

					err := layer.Validate()
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		for _, testCase := range []ValidatorErrorCase{
			{
				"url",
				&models.ImageLayer{
					DestinationPath: "local_location",
				},
			},
			{
				"destination_path",
				&models.ImageLayer{
					Url: "web_location",
				},
			},
			{
				"layer_type",
				&models.ImageLayer{},
			},
			{
				"layer_type",
				&models.ImageLayer{
					LayerType: models.ImageLayer_Type(10),
				},
			},
			{
				"digest_value",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					DigestAlgorithm: models.DigestAlgorithmSha256,
					MediaType:       models.MediaTypeTgz,
				},
			},
			{
				"digest_algorithm",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					DigestValue:     "some digest",
					MediaType:       models.MediaTypeTgz,
				},
			},
			{
				"digest_value",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					MediaType:       models.MediaTypeTgz,
					LayerType:       models.LayerTypeExclusive,
				},
			},
			{
				"digest_algorithm",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					MediaType:       models.MediaTypeTgz,
					LayerType:       models.LayerTypeExclusive,
				},
			},
			{
				"digest_algorithm",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					DigestAlgorithm: models.ImageLayer_DigestAlgorithm(5),
					DigestValue:     "some digest",
					MediaType:       models.MediaTypeTgz,
				},
			},
			{
				"media_type",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					DigestAlgorithm: models.DigestAlgorithmSha256,
					DigestValue:     "some digest",
				},
			},
			{
				"media_type",
				&models.ImageLayer{
					Url:             "web_location",
					DestinationPath: "local_location",
					DigestAlgorithm: models.DigestAlgorithmSha256,
					DigestValue:     "some digest",
					MediaType:       models.ImageLayer_MediaType(9),
				},
			},
		} {
			testValidatorErrorCase(testCase)
		}
	})
})
