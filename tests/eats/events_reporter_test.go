package eats_test

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventsReporter", func() {
	var (
		guid    string
		version string
		image   string
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		version = tests.GenerateGUID()
		image = "eirini/notdora"

		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
			},
			Response: wiremock.Response{
				Status: 200,
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		statusCode := desireLRP(cf.DesireLRPRequest{
			Namespace:    fixture.Namespace,
			GUID:         guid,
			Version:      version,
			NumInstances: 1,
			DiskMB:       512,
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: image,
				},
			},
		})
		Expect(statusCode).To(Equal(http.StatusAccepted))
	})

	AfterEach(func() {
		_, err := stopLRP(guid, version)
		Expect(err).NotTo(HaveOccurred())
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
				URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
			}

			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").ShouldNot(BeZero())
			currentCount, err := fixture.Wiremock.GetCount(requestMatcher)
			Expect(err).NotTo(HaveOccurred())
			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").Should(BeNumerically(">", currentCount))
		})
	})
})
