package eats_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks Reporter", func() {
	var (
		taskRequest        cf.TaskRequest
		taskGUID           string
		callbackStatusCode int
	)

	BeforeEach(func() {

		taskGUID = tests.GenerateGUID()
		callbackStatusCode = http.StatusOK

		taskRequest = cf.TaskRequest{
			GUID:               taskGUID,
			Namespace:          fixture.Namespace,
			Name:               "some-task",
			AppGUID:            tests.GenerateGUID(),
			AppName:            "some-app",
			OrgName:            "the-org",
			SpaceName:          "the-space",
			CompletionCallback: fmt.Sprintf("%s/%s", fixture.Wiremock.URL, taskGUID),
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: "busybox",
					Command: []string{
						"bin/sleep",
						"10",
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/%s", taskGUID),
			},
			Response: wiremock.Response{
				Status: callbackStatusCode,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		desireOpiTask(taskRequest)
		Eventually(jobExists(taskRequest.GUID)).Should(BeTrue())
	})

	It("deletes the task after it completes", func() {
		Eventually(jobExists(taskRequest.GUID)).Should(BeFalse())
	})

	It("notifies the cloud controller", func() {
		requestMatcher := wiremock.RequestMatcher{
			Method: "POST",
			URL:    fmt.Sprintf("/%s", taskGUID),
		}
		Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").Should(Equal(1))

		bodyStr, err := fixture.Wiremock.GetRequestBody(requestMatcher)
		Expect(err).NotTo(HaveOccurred())

		var request cf.TaskCompletedRequest
		err = json.Unmarshal([]byte(bodyStr), &request)
		Expect(err).NotTo(HaveOccurred())

		Expect(request.TaskGUID).To(Equal(taskGUID))
		Expect(request.Failed).To(BeFalse())
		Expect(request.FailureReason).To(BeEmpty())
	})

	When("posting to the cloud controller fails", func() {
		BeforeEach(func() {
			callbackStatusCode = http.StatusTeapot
		})

		It("does not delete the job", func() {
			requestMatcher := wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/%s", taskGUID),
			}
			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").Should(Equal(1))

			Expect(jobExists(taskRequest.GUID)()).To(BeTrue())
		})

	})
})

func jobExists(guid string) func() bool {
	return func() bool {
		for _, job := range listJobs() {
			if job.Spec.Template.Annotations[k8s.AnnotationGUID] == guid {
				return true
			}
		}

		return false
	}
}

func desireOpiTask(taskRequest cf.TaskRequest) {
	data, err := json.Marshal(taskRequest)
	Expect(err).NotTo(HaveOccurred())

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", tests.GetEiriniAddress(), taskRequest.GUID), bytes.NewReader(data))
	Expect(err).NotTo(HaveOccurred())

	response, err := fixture.GetEiriniHTTPClient().Do(request)
	Expect(err).NotTo(HaveOccurred())

	defer response.Body.Close()

	Expect(response).To(HaveHTTPStatus(http.StatusAccepted))
}
