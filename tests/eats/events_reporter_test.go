package eats_test

import (
	"fmt"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventsReporter", func() {
	var (
		guid  string
		image string
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		image = "eirini/notdora"

		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/internal/v4/apps/%s-1/crashed", guid),
			},
			Response: wiremock.Response{
				Status: 200,
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		desireLRP(cf.DesireLRPRequest{
			Namespace:    fixture.Namespace,
			GUID:         guid,
			Version:      "1",
			NumInstances: 1,
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: image,
				},
			},
		})
	})

	It("does not report a crash event for running apps", func() {
		requestMatcher := wiremock.RequestMatcher{
			Method: "POST",
			URL:    fmt.Sprintf("/internal/v4/apps/%s-1/crashed", guid),
		}

		Consistently(fixture.Wiremock.GetCountFn(requestMatcher), "10s").Should(BeZero())
	})

	When("the app crashes", func() {
		BeforeEach(func() {
			image = "eirini/icarus"
		})

		It("reports a crash event", func() {
			requestMatcher := wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/internal/v4/apps/%s-1/crashed", guid),
			}

			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "10s").ShouldNot(BeZero())
			currentCount, err := fixture.Wiremock.GetCount(requestMatcher)
			Expect(err).NotTo(HaveOccurred())
			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").Should(BeNumerically(">", currentCount))
		})
	})
})
