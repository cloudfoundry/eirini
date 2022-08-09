package eats_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventsReporter [needs-logs-for: eirini-api, eirini-event-reporter, cc-wiremock]", func() {
	var (
		guid            string
		version         string
		appServiceName  string
		expectedRequest wiremock.RequestMatcher
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		version = tests.GenerateGUID()

		expectedRequest = wiremock.RequestMatcher{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
		}
		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: expectedRequest,
			Response: wiremock.Response{
				Status: 200,
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	When("the app starts successfully", func() {
		BeforeEach(func() {
			statusCode := desireLRP(cf.DesireLRPRequest{
				Namespace:    fixture.Namespace,
				GUID:         guid,
				Version:      version,
				NumInstances: 1,
				DiskMB:       512,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/dorini",
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))

			appServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, guid, 8080, "/")
		})

		It("does not report a crash event for running apps", func() {
			Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(BeZero())
		})

		When("the app exits with non-zero code", func() {
			BeforeEach(func() {
				_, err := http.Get(fmt.Sprintf("http://%s.%s:8080/exit?exitCode=1", appServiceName, fixture.Namespace))
				Expect(err).To(MatchError(ContainSubstring("EOF"))) // The app exited
			})

			It("reports a crash event", func() {
				Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(Equal(1))
				Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(Equal(1))

				verifyCrashRequest(expectedRequest, 1)
			})
		})

		When("the app exits with zero code", func() {
			BeforeEach(func() {
				_, err := http.Get(fmt.Sprintf("http://%s.%s:8080/exit?exitCode=0", appServiceName, fixture.Namespace))
				Expect(err).To(MatchError(ContainSubstring("EOF"))) // The app exited
			})

			It("reports a crash event", func() {
				Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(Equal(1))
				Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(Equal(1))

				verifyCrashRequest(expectedRequest, 0)
			})
		})
	})

	When("the app is crashing on startup", func() {
		BeforeEach(func() {
			_, err := stopLRP(guid, version)
			Expect(err).NotTo(HaveOccurred())

			statusCode := desireLRP(cf.DesireLRPRequest{
				Namespace:    fixture.Namespace,
				GUID:         guid,
				Version:      version,
				NumInstances: 1,
				DiskMB:       512,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/busybox",
						Command:          []string{"bad command"},
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))
		})

		It("reports a crash event per app restart", func() {
			Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(BeNumerically(">", 1))
		})
	})
})

func verifyCrashRequest(requestMatcher wiremock.RequestMatcher, exitStatus int) {
	body, err := fixture.Wiremock.GetRequestBody(requestMatcher)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	var request cc_messages.AppCrashedRequest
	err = json.Unmarshal([]byte(body), &request)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	ExpectWithOffset(1, request.ExitStatus).To(Equal(exitStatus))
	ExpectWithOffset(1, request.CrashCount).To(Equal(1))
}
