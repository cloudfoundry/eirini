package k8s_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/k8s"
	"k8s.io/api/core/v1"
)

var _ = Describe("Helpers", func() {

	Context("Map to Kuberentes EnvVar", func() {

		var (
			env     map[string]string
			envVars []v1.EnvVar
		)

		BeforeEach(func() {
			env = map[string]string{
				"foo":  "bar",
				"dora": "fedora",
			}
		})

		JustBeforeEach(func() {
			envVars = MapToEnvVar(env)
		})

		It("translates key-values to EnvVars", func() {
			Expect(envVars).To(ConsistOf(v1.EnvVar{Name: "foo", Value: "bar"}, v1.EnvVar{Name: "dora", Value: "fedora"}))
		})

		Context("when env map is empty", func() {

			BeforeEach(func() {
				env = map[string]string{}
			})

			It("should return an empty slice", func() {
				Expect(envVars).To(BeEmpty())
			})
		})
	})
})
