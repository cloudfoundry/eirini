package opi_test

import (
	"bytes"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desire Task", func() {
	var body string

	BeforeEach(func() {
		body = `{
			"guid": "the-task-guid",
			"version": "0.0.0",
			"ports" : [8080],
			"command" : ["echo", "hi"],
			"environment": [
			   {
				   "name": "my-env",
					 "value": "my-value"
				 }
			],
		  "lifecycle": {
				"buildpack_lifecycle": {
					"droplet_guid": "dropla",
					"droplet_hash": "mash",
					"start_command": "cmd"
				}
			}
		}`
	})

	JustBeforeEach(func() {
		desireTaskReq, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/the-task-guid", url), bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		resp, err := httpClient.Do(desireTaskReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("should create a job for the task", func() {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(jobs.Items).To(HaveLen(1))
		Expect(jobs.Items[0].Name).To(Equal("the-task-guid"))

		jobContainers := jobs.Items[0].Spec.Template.Spec.Containers
		Expect(jobContainers).To(HaveLen(1))
		Expect(jobContainers[0].Command).To(ConsistOf("echo", "hi"))
		Expect(jobContainers[0].Env).To(ContainElement(corev1.EnvVar{Name: "my-env", Value: "my-value"}))
	})
})
