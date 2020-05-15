package eats_test

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		taskReporterConfigFile string
		taskReporterSession    *gexec.Session

		task cf.TaskRequest
	)

	BeforeEach(func() {

		config := &eirini.ReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			EiriniAddress:  opiURL,
			EiriniCertPath: localhostCertPath,
			CAPath:         localhostCertPath,
			EiriniKeyPath:  localhostKeyPath,
		}

		taskReporterSession, taskReporterConfigFile = util.RunBinary(binPaths.TaskReporter, config)
	})

	AfterEach(func() {
		if taskReporterSession != nil {
			taskReporterSession.Kill()
		}
		Expect(os.Remove(taskReporterConfigFile)).To(Succeed())
	})

	Context("When an task is created", func() {
		BeforeEach(func() {
		})

		It("cleans up the job after it completes", func() {
			By("creating the task", func() {
				task = cf.TaskRequest{
					GUID: "the-task",
					Lifecycle: cf.Lifecycle{
						DockerLifecycle: &cf.DockerLifecycle{
							Image:   "busybox",
							Command: []string{"sh", "-c", "sleep 1; echo something to stdout"},
						},
					},
				}
				resp, err := desireTask(httpClient, opiURL, task)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(getTaskJobsFn("the-task")).Should(HaveLen(1))
			})

			By("cleaning up the task", func() {
				Eventually(getTaskJobsFn("the-task")).Should(BeEmpty())
			})
		})
	})
})

func getTaskJobsFn(guid string) func() ([]batchv1.Job, error) {
	return func() ([]batchv1.Job, error) {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s, %s=%s", k8s.LabelSourceType, "TASK", k8s.LabelGUID, guid),
		})
		return jobs.Items, err
	}
}
