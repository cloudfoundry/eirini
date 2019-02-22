package util_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/eirini/util"
)

var _ = Describe("Parse", func() {
	It("should parse app name and index from pod name", func() {
		appName, index, err := util.ParseAppNameAndIndex("some-name-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(appName).To(Equal("somename"))
		Expect(index).To(Equal(1))
	})

	Context("when the pod name does not contain dashes", func() {
		It("should return an error", func() {
			appName, index, err := util.ParseAppNameAndIndex("somename1")

			Expect(err).To(HaveOccurred())
			Expect(appName).To(Equal(""))
			Expect(index).To(Equal(0))
		})

	})

	Context("when the last part in pod name is not a number", func() {
		It("should return an error", func() {
			appName, index, err := util.ParseAppNameAndIndex("somename-a")

			Expect(err).To(HaveOccurred())
			Expect(appName).To(Equal(""))
			Expect(index).To(Equal(0))
		})

	})

})
