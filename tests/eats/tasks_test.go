package eats_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		guid string
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		desireTask(cf.TaskRequest{
			GUID:               guid,
			Namespace:          fixture.Namespace,
			CompletionCallback: "http://example.com/",
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: "busybox",
					Command: []string{
						"bin/sleep",
						"10",
					},
				},
			},
		})
	})

	Describe("Running a task", func() {
		It("creates a job", func() {
			job := getJob(guid)
			Expect(job).NotTo(BeNil())
			Expect(job.Labels[k8s.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("Getting a task", func() {
		It("returns the task", func() {
			taskResponse, err := getTask(guid)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskResponse.GUID).To(Equal(guid))
		})

		It("returns an error for a non existing task GUID", func() {
			_, err := getTask(tests.GenerateGUID())
			Expect(err).To(MatchError("404 Not Found"))
		})
	})

	Describe("Cancelling a task", func() {
		It("deletes the job", func() {
			Expect(cancelTask(guid)).To(Succeed())
			Expect(getJob(guid)).To(BeNil())
		})

		It("returns an error on cancelling a non-existent task", func() {
			Expect(cancelTask(tests.GenerateGUID())).To(MatchError("500 Internal Server Error"))
		})
	})

	Describe("Listing tasks", func() {
		It("lists", func() {
			tasks, err := listTasks()
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks).NotTo(BeEmpty())

			taskGUIDs := []string{}
			for _, t := range tasks {
				taskGUIDs = append(taskGUIDs, t.GUID)
			}
			Expect(taskGUIDs).To(ContainElement(guid))
		})
	})
})

func httpDo(method, url string) (*http.Response, error) {
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	response, err := fixture.GetEiriniHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 400 {
		defer response.Body.Close()

		return nil, errors.New(response.Status)
	}

	return response, nil
}

func getTask(guid string) (cf.TaskResponse, error) {
	response, err := httpDo("GET", fmt.Sprintf("%s/tasks/%s", tests.GetEiriniAddress(), guid))
	if err != nil {
		return cf.TaskResponse{}, err
	}

	defer response.Body.Close()

	var taskResponse cf.TaskResponse
	if err := json.NewDecoder(response.Body).Decode(&taskResponse); err != nil {
		return cf.TaskResponse{}, err
	}

	return taskResponse, nil
}

func listTasks() ([]cf.TaskResponse, error) {
	response, err := httpDo("GET", fmt.Sprintf("%s/tasks", tests.GetEiriniAddress()))
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	var taskResponses []cf.TaskResponse
	if err := json.NewDecoder(response.Body).Decode(&taskResponses); err != nil {
		return nil, err
	}

	return taskResponses, nil
}

func cancelTask(guid string) error {
	response, err := httpDo("DELETE", fmt.Sprintf("%s/tasks/%s", tests.GetEiriniAddress(), guid))
	if err != nil {
		return err
	}

	defer response.Body.Close()

	return nil
}

func getJob(taskGUID string) *batchv1.Job {
	jobs, err := fixture.Clientset.BatchV1().Jobs("").List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", k8s.LabelGUID, taskGUID),
	})
	Expect(err).NotTo(HaveOccurred())

	if len(jobs.Items) == 0 {
		return nil
	}

	Expect(jobs.Items).To(HaveLen(1))

	return &jobs.Items[0]
}
