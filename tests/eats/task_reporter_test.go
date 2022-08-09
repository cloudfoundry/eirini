package eats_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks Reporter [needs-logs-for: eirini-api, eirini-task-reporter]", func() {
	var (
		taskRequest        cf.TaskRequest
		taskGUID           string
		callbackStatusCode int
		taskServiceName    string
		port               int32
	)

	BeforeEach(func() {
		taskGUID = tests.GenerateGUID()
		callbackStatusCode = http.StatusOK

		taskRequest = cf.TaskRequest{
			GUID:               taskGUID,
			Namespace:          fixture.Namespace,
			AppGUID:            tests.GenerateGUID(),
			CompletionCallback: fmt.Sprintf("%s/%s", fixture.Wiremock.Address(), taskGUID),
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: "eirini/dorini",
				},
			},
		}

		port = 8080
	})

	JustBeforeEach(func() {
		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: wiremock.RequestMatcher{
				Method: http.MethodPost,
				URL:    fmt.Sprintf("/%s", taskGUID),
			},
			Response: wiremock.Response{
				Status: callbackStatusCode,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		desireTask(taskRequest)

		taskServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, taskGUID, port, "/")

		// Make sure the task eventually completes
		_, _ = tests.RequestServiceFn(fixture.Namespace, taskServiceName, port, "/exit")()
	})

	It("notifies the cloud controller", func() {
		requestMatcher := wiremock.RequestMatcher{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("/%s", taskGUID),
		}
		Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "2m").Should(Equal(1))

		bodyStr, err := fixture.Wiremock.GetRequestBody(requestMatcher)
		Expect(err).NotTo(HaveOccurred())

		var request cf.TaskCompletedRequest
		err = json.Unmarshal([]byte(bodyStr), &request)
		Expect(err).NotTo(HaveOccurred())

		Expect(request.TaskGUID).To(Equal(taskGUID))
		Expect(request.Failed).To(BeFalse())
		Expect(request.FailureReason).To(BeEmpty())
	})

	When("posting to the cloud controller continuously fails", func() {
		BeforeEach(func() {
			callbackStatusCode = http.StatusTeapot
		})

		It("retries", func() {
			requestMatcher := wiremock.RequestMatcher{
				Method: http.MethodPost,
				URL:    fmt.Sprintf("/%s", taskGUID),
			}
			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").Should(BeNumerically(">", 1))
		})
	})
})
