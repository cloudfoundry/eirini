package models_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricTagValue", func() {
	Describe("Validate", func() {
		It("is valid when there is only a static value specified", func() {
			value := &models.MetricTagValue{
				Static: "some-value",
			}
			Expect(value.Validate()).To(Succeed())
		})

		It("is valid when there is only a dynamic value specified", func() {
			value := &models.MetricTagValue{
				Dynamic: models.MetricTagDynamicValueIndex,
			}
			Expect(value.Validate()).To(Succeed())
		})

		It("is not valid when there is an invalid dynamic value specified", func() {
			value := &models.MetricTagValue{
				Dynamic: 100,
			}
			Expect(value.Validate()).To(MatchError(ContainSubstring("dynamic")))
		})

		It("is not valid when both static and dynamic values are specified", func() {
			value := &models.MetricTagValue{
				Static:  "some-value",
				Dynamic: models.MetricTagDynamicValueIndex,
			}
			err := value.Validate()
			Expect(err).To(MatchError(ContainSubstring("static")))
			Expect(err).To(MatchError(ContainSubstring("dynamic")))
		})

		It("is not valid when neither static or dynamic values are specified", func() {
			value := &models.MetricTagValue{}
			err := value.Validate()
			Expect(err).To(MatchError(ContainSubstring("static")))
			Expect(err).To(MatchError(ContainSubstring("dynamic")))
		})
	})
})
