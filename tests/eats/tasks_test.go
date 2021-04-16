package eats_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks [needs-logs-for: eirini-api, eirini-task-reporter]", func() {
	var (
		guid               string
		completionCallback string
		taskServiceName    string
		port               int32
	)

	BeforeEach(func() {
		completionCallback = "http://example.com/"
		guid = tests.GenerateGUID()
		port = 8080
	})

	JustBeforeEach(func() {
		desireTask(cf.TaskRequest{
			GUID:               guid,
			Namespace:          fixture.Namespace,
			CompletionCallback: completionCallback,
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: "eirini/dorini",
				},
			},
		})

		taskServiceName = exposeAsService(fixture.Namespace, guid, port)
	})

	It("runs the task", func() {
		Eventually(requestServiceFn(fixture.Namespace, taskServiceName, port, "/")).Should(ContainSubstring("Dora!"))
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
		It("kills the task", func() {
			// better to check Task status here, once that is available
			Eventually(func() error {
				_, err := requestServiceFn(fixture.Namespace, taskServiceName, port, "/")()

				return err
			}, "20s").Should(HaveOccurred())
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
