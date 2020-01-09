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

var _ = Describe("Staging", func() {

	BeforeEach(func() {
		body := `{
				"memory_mb": 100,
				"disk_mb": 200,
				"cpu_weight": 50
			}`
		resp, err := httpClient.Post(fmt.Sprintf("%s/stage/the-staging-guid", url), "json", bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("should create a staging job", func() {
		jobs, err := clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(jobs.Items).Should(HaveLen(1))
		Expect(jobs.Items[0].Name).Should(Equal("the-staging-guid"))
	})

	It("should create the correct containers for the job", func() {
		jobs, err := clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		job := jobs.Items[0]
		Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
		Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("opi-task-downloader"))
		Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("opi-task-executor"))

		Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("opi-task-uploader"))
	})

	It("should set the correct job resource requirements", func() {
		jobs, err := clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		job := jobs.Items[0]

		memoryResourceRequest := job.Spec.Template.Spec.InitContainers[1].Resources.Requests[corev1.ResourceMemory]
		Expect(memoryResourceRequest.String()).To(Equal("100M"))

		cpuResourceRequest := job.Spec.Template.Spec.InitContainers[1].Resources.Requests[corev1.ResourceCPU]
		Expect(cpuResourceRequest.String()).To(Equal("500m"))

		diskResourceRequest := job.Spec.Template.Spec.InitContainers[1].Resources.Requests[corev1.ResourceEphemeralStorage]
		Expect(diskResourceRequest.String()).To(Equal("200M"))
	})

})
