package opi_test

import (
	"bytes"
	"fmt"
	"net/http"

	. "code.cloudfoundry.org/eirini/k8s"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Staging", func() {

	var (
		job  batch.Job
		body string
	)

	BeforeEach(func() {
		body = `{
				"app_name": "my_app",
				"space_name": "my_space",
				"lifecycle": {
					"buildpack_lifecycle": {}
				}
			}`
	})

	JustBeforeEach(func() {
		resp, err := httpClient.Post(fmt.Sprintf("%s/stage/the-staging-guid", url), "json", bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		job = jobs.Items[0]
	})

	It("should create a staging job", func() {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(jobs.Items).Should(HaveLen(1))
		Expect(jobs.Items[0].Name).Should(HavePrefix("my-app-my-space-"))
	})

	It("should create the correct containers for the job", func() {
		Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
		Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("opi-task-downloader"))
		Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("opi-task-executor"))

		Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("opi-task-uploader"))
	})

	Context("when resource constrains are provided", func() {

		BeforeEach(func() {
			body = `{
				"memory_mb": 100,
				"disk_mb": 200,
				"cpu_weight": 50,
				"lifecycle": {
					"buildpack_lifecycle": {}
				}
			}`
		})

		It("should set the correct job resource requirements", func() {
			memoryResourceRequest := job.Spec.Template.Spec.InitContainers[1].Resources.Requests[corev1.ResourceMemory]
			Expect(memoryResourceRequest.String()).To(Equal("100M"))

			cpuResourceRequest := job.Spec.Template.Spec.InitContainers[1].Resources.Requests[corev1.ResourceCPU]
			Expect(cpuResourceRequest.String()).To(Equal("500m"))

			diskResourceRequest := job.Spec.Template.Spec.InitContainers[1].Resources.Requests[corev1.ResourceEphemeralStorage]
			Expect(diskResourceRequest.String()).To(Equal("200M"))
		})
	})

	Context("when app information is provided", func() {

		BeforeEach(func() {
			body = `{
				"app_name": "my-app",
				"app_guid": "my-app-guid",
				"org_name": "my-org",
				"org_guid": "org-id",
				"space_name": "my-space",
				"space_guid": "space-id",
				"lifecycle": {
					"buildpack_lifecycle": {}
				}
			}`

		})

		DescribeTable("it should add Annotations accordingly", func(key, value string) {
			Expect(job.Annotations[key]).To(Equal(value))
		},
			Entry("AppName", AnnotationAppName, "my-app"),
			Entry("AppGUID", AnnotationAppID, "my-app-guid"),
			Entry("OrgName", AnnotationOrgName, "my-org"),
			Entry("OrgName", AnnotationOrgGUID, "org-id"),
			Entry("SpaceName", AnnotationSpaceName, "my-space"),
			Entry("SpaceGUID", AnnotationSpaceGUID, "space-id"),
		)

		DescribeTable("it should add labels", func(key, value string) {
			Expect(job.Labels[key]).To(Equal(value))
		},
			Entry("AppGUID", LabelAppGUID, "my-app-guid"),
			Entry("LabelGUID", LabelGUID, "the-staging-guid"),
			Entry("SourceType", LabelSourceType, "STG"),
			Entry("StagingGUID", LabelStagingGUID, "the-staging-guid"),
		)
	})
})
