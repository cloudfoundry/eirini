package eats_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks Reporter [needs-logs-for: eirini-api, eirini-task-reporter]", func() {
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
			AppGUID:            tests.GenerateGUID(),
			CompletionCallback: fmt.Sprintf("%s/%s", fixture.Wiremock.URL, taskGUID),
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: "eirini/busybox",
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

		desireTask(taskRequest)
		Eventually(jobExists(taskGUID)).Should(BeTrue())
	})

	AfterEach(func() {
		Expect(cleanupJob(taskGUID)).To(Succeed())
	})

	It("deletes the task after it completes", func() {
		Eventually(jobExists(taskGUID)).Should(BeFalse())
	})

	It("notifies the cloud controller", func() {
		requestMatcher := wiremock.RequestMatcher{
			Method: "POST",
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

		It("deletes the job after a number of attempts at the callback", func() {
			requestMatcher := wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/%s", taskGUID),
			}
			Eventually(fixture.Wiremock.GetCountFn(requestMatcher), "1m").Should(BeNumerically(">", 1))
			Eventually(jobExists(taskGUID)).Should(BeFalse())
		})
	})
})

func jobExists(guid string) func() bool {
	return func() bool {
		jobs := listJobs(guid)

		return len(jobs) > 0
	}
}

func listJobs(guid string) []batchv1.Job {
	jobs, err := fixture.Clientset.
		BatchV1().
		Jobs(fixture.Namespace).
		List(context.Background(),
			metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", k8s.LabelGUID, guid),
			},
		)

	Expect(err).NotTo(HaveOccurred())

	return jobs.Items
}

func cleanupJob(guid string) error {
	bgDelete := metav1.DeletePropagationBackground

	return fixture.Clientset.
		BatchV1().
		Jobs(fixture.Namespace).
		DeleteCollection(
			context.Background(),
			metav1.DeleteOptions{PropagationPolicy: &bgDelete},
			metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", k8s.LabelGUID, guid)},
		)
}
