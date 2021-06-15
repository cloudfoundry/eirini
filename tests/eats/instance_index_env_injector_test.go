package eats_test

import (
	"net/http"
	"regexp"

	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstanceIndexEnvInjector [needs-logs-for: eirini-api, instance-index-env-injector]", func() {
	var appServiceName string

	BeforeEach(func() {
		lrpGUID := tests.GenerateGUID()
		lrpVersion := tests.GenerateGUID()

		lrp := createLrpRequest(lrpGUID, lrpVersion)
		lrp.Namespace = fixture.Namespace
		lrp.NumInstances = 3
		statusCode := desireLRP(lrp)
		Expect(statusCode).To(Equal(http.StatusAccepted))

		appServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, lrpGUID, 8080, "/")
	})

	It("creates pods with CF_INSTANCE_INDEX set to 0, 1 and 2", func() {
		guids := map[string]bool{}
		re := regexp.MustCompile(`CF_INSTANCE_INDEX=(.*)`)
		Eventually(func() int {
			envvars, err := tests.RequestServiceFn(fixture.Namespace, appServiceName, 8080, "/env")()
			if err != nil {
				return 0
			}
			matches := re.FindStringSubmatch(envvars)
			if len(matches) == 2 {
				guids[matches[1]] = true
			}

			return len(guids)
		}).Should(Equal(3))

		Expect(guids).To(And(HaveKey("0"), HaveKey("1"), HaveKey("2")))
	})
})
