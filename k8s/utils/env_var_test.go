package utils_test

import (
	"code.cloudfoundry.org/eirini/k8s/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("EnvVar", func() {
	It("returns the env value", func() {
		value, err := utils.GetEnvVarValue("foo", []corev1.EnvVar{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "boo"},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(Equal("bar"))
	})

	When("the env value does not exist", func() {
		It("fails", func() {
			_, err := utils.GetEnvVarValue("foo", []corev1.EnvVar{
				{Name: "baz", Value: "boo"},
			})

			Expect(err).To(MatchError("failed to find env var"))
		})
	})
})
