package opi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Staging", func() {

	var (
		job         batch.Job
		body        string
		stagingGUID string
	)

	BeforeEach(func() {
		body = `{
				"app_name": "my_app",
				"space_name": "my_space",
				"lifecycle": {
					"buildpack_lifecycle": {}
				}
			}`
		stagingGUID = "the-staging-guid"
	})

	JustBeforeEach(func() {
		resp, err := httpClient.Post(fmt.Sprintf("%s/stage/%s", url, stagingGUID), "json", bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		job = jobs.Items[0]
	})

	It("should create a staging job", func() {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(jobs.Items).Should(HaveLen(1))
		Expect(jobs.Items[0].Name).Should(HavePrefix("my-app-my-space"))
		Expect(jobs.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal("staging"))
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

	var _ = Describe("staging completion", func() {
		var (
			cloudControllerServer *ghttp.Server
			completionRequest     cf.StagingCompletedRequest
			body                  []byte
			resp                  *http.Response
			rawResult             json.RawMessage
		)

		BeforeEach(func() {
			stagingGUID = "completion-guid-1"
			var err error
			cloudControllerServer, err = util.CreateTestServer(certPath, keyPath, certPath)
			Expect(err).ToNot(HaveOccurred())
			cloudControllerServer.HTTPTestServer.StartTLS()

			rawResult = json.RawMessage(`{"very":"good"}`)
			cloudControllerServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
					ghttp.VerifyJSONRepresenting(cc_messages.StagingResponseForCC{
						Result: &rawResult,
					}),
				),
			)
		})

		AfterEach(func() {
			cloudControllerServer.Close()
		})

		JustBeforeEach(func() {
			annotation := cc_messages.StagingTaskAnnotation{
				CompletionCallback: cloudControllerServer.URL(),
			}
			annotationJSON, err := json.Marshal(annotation)
			Expect(err).NotTo(HaveOccurred())

			completionRequest = cf.StagingCompletedRequest{
				TaskGUID:   stagingGUID,
				Failed:     false,
				Annotation: string(annotationJSON),
				Result:     `{"very":"good"}`,
			}

			body, err = json.Marshal(completionRequest)
			Expect(err).NotTo(HaveOccurred())
			req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/stage/%s/completed", url, stagingGUID), bytes.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			resp, err = httpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass the Result through to the CC staging completed endpoint", func() {
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		})

		When("CC TLS is disabled and CC certs not configured", func() {
			var (
				newConfigPath string
			)

			BeforeEach(func() {
				newConfigPath = restartWithConfig(func(cfg eirini.Config) eirini.Config {
					cfg.CCTLSDisabled = true
					cfg.CCCertPath = ""
					cfg.CCKeyPath = ""
					cfg.CCCAPath = ""

					return cfg
				})
				stagingGUID = "completion-guid-2"

				cloudControllerServer.Close()
				cloudControllerServer = ghttp.NewServer()
				cloudControllerServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSONRepresenting(cc_messages.StagingResponseForCC{
							Result: &rawResult,
						}),
					),
				)
			})

			AfterEach(func() {
				os.RemoveAll(newConfigPath)
			})

			It("should invoke the CC staging completed endpoint", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
			})
		})
	})
})
