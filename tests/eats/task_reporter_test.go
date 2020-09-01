package eats_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tasks Reporter", func() {
	var (
		session     *gexec.Session
		configFile  string
		taskRequest cf.TaskRequest
	)

	BeforeEach(func() {
		config := &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			CCTLSDisabled: true,
		}
		session, configFile = eiriniBins.TaskReporter.Run(config)

		taskRequest = cf.TaskRequest{
			GUID:               tests.GenerateGUID(),
			Namespace:          fixture.Namespace,
			Name:               "some-task",
			AppGUID:            tests.GenerateGUID(),
			AppName:            "some-app",
			OrgName:            "the-org",
			SpaceName:          "the-space",
			CompletionCallback: "http://example.com/complete",
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

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		Expect(os.Remove(configFile)).To(Succeed())
	})

	JustBeforeEach(func() {
		desireOpiTask(taskRequest)
		Eventually(jobExists(taskRequest.GUID)).Should(BeTrue())
	})

	It("deletes the task after it completes", func() {
		Eventually(jobExists(taskRequest.GUID)).Should(BeFalse())
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

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", opiURL, taskRequest.GUID), bytes.NewReader(data))
	Expect(err).NotTo(HaveOccurred())

	response, err := httpClient.Do(request)
	Expect(err).NotTo(HaveOccurred())

	defer response.Body.Close()

	Expect(response).To(HaveHTTPStatus(http.StatusAccepted))
}
