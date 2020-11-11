package eats_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventsReporter", func() {
	var (
		guid            string
		version         string
		expectedRequest wiremock.RequestMatcher
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		version = tests.GenerateGUID()

		expectedRequest = wiremock.RequestMatcher{
			Method: "POST",
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

	AfterEach(func() {
		_, err := stopLRP(guid, version)
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
		})

		It("does not report a crash event for running apps", func() {
			Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(BeZero())
		})
	})

	When("the app exists with non-zero code", func() {
		BeforeEach(func() {
			statusCode := desireLRP(cf.DesireLRPRequest{
				Namespace:    fixture.Namespace,
				GUID:         guid,
				Version:      version,
				NumInstances: 1,
				DiskMB:       512,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/busybox",
						Command:          []string{"/bin/sh", "-c", "exit 1"},
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))
		})

		It("reports a crash event", func() {
			Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(BeNumerically(">=", 1))
			verifyCrashRequest(expectedRequest, 1)
		})
	})

	When("the app exists with zero code", func() {
		BeforeEach(func() {
			statusCode := desireLRP(cf.DesireLRPRequest{
				Namespace:    fixture.Namespace,
				GUID:         guid,
				Version:      version,
				NumInstances: 1,
				DiskMB:       512,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/busybox",
						Command:          []string{"/bin/sh", "-c", "exit 0"},
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))
		})

		It("reports a crash event", func() {
			Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(BeNumerically(">=", 1))
			verifyCrashRequest(expectedRequest, 0)
		})
	})

	When("the app is crashing on startup", func() {
		BeforeEach(func() {
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
	Expect(err).NotTo(HaveOccurred())

	var request cc_messages.AppCrashedRequest
	err = json.Unmarshal([]byte(body), &request)
	Expect(err).NotTo(HaveOccurred())

	Expect(request.ExitStatus).To(Equal(exitStatus))
	Expect(request.CrashCount).To(BeNumerically(">=", 1))
}
