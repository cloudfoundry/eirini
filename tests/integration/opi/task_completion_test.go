package opi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks completion", func() {
	var (
		taskReporterConfig  *eirini.TaskReporterConfig
		taskReporterSession *gexec.Session
		taskGUID            string
	)

	BeforeEach(func() {
		taskReporterConfig = &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
			CCCertPath:                   certPath,
			CAPath:                       certPath,
			CCKeyPath:                    keyPath,
			CompletionCallbackRetryLimit: 3,
			TTLSeconds:                   600,
		}
	})

	JustBeforeEach(func() {
		taskReporterSession, _ = eiriniBins.TaskReporter.Run(taskReporterConfig)

		taskGUID = tests.GenerateGUID()
		taskRequest := cf.TaskRequest{
			GUID:      taskGUID,
			AppName:   "my_app",
			SpaceName: "my_space",
			Namespace: fixture.Namespace,
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image:   "eirini/busybox",
					Command: []string{"/bin/sleep", "1"},
				},
			},
			CompletionCallback: "http://example.com",
		}
		response, err := httpDo("POST", fmt.Sprintf("%s/tasks/%s", url, taskGUID), taskRequest)
		Expect(err).NotTo(HaveOccurred())
		defer response.Body.Close()
	})

	AfterEach(func() {
		Eventually(taskReporterSession.Terminate()).Should(gexec.Exit())
	})

	It("does not list completed tasks that have not reached their ttl", func() {
		Eventually(listTasks).Should(BeEmpty())
		Expect(getJob(taskGUID)).NotTo(BeNil())
	})

	It("does not get a completed tasks that has not reached its ttl", func() {
		Eventually(func() error {
			_, err := getTask(taskGUID)

			return err
		}).Should(MatchError(ContainSubstring("404 Not Found")))
		Expect(getJob(taskGUID)).NotTo(BeNil())
	})
})

func listTasks() ([]cf.TaskResponse, error) {
	response, err := httpDo("GET", fmt.Sprintf("%s/tasks", url), nil)
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

func getTask(guid string) (cf.TaskResponse, error) {
	response, err := httpDo("GET", fmt.Sprintf("%s/tasks/%s", url, guid), nil)
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

func httpDo(method, url string, b interface{}) (*http.Response, error) {
	body, err := json.Marshal(b)
	Expect(err).NotTo(HaveOccurred())

	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 400 {
		defer response.Body.Close()

		return nil, errors.New(response.Status)
	}

	return response, nil
}
