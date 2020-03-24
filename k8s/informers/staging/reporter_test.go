package staging_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/staging"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("FailedStagingReporter", func() {

	var (
		reporter staging.FailedStagingReporter
		server   *ghttp.Server
		logger   *lagertest.TestLogger
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		logger = lagertest.NewTestLogger("staging-reporter-test")

		reporter = staging.FailedStagingReporter{
			Client: &http.Client{},
			Logger: logger,
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Reporting status to Eirini", func() {

		var (
			thePod                               *v1.Pod
			containerStatus, initContainerStatus v1.ContainerStatus
			env                                  []v1.EnvVar
		)

		BeforeEach(func() {
			env = []v1.EnvVar{
				{
					Name:  "COMPLETION_CALLBACK",
					Value: "internal_cc_staging_endpoint.io/stage/build_completed",
				},
				{
					Name:  "EIRINI_ADDRESS",
					Value: server.URL(),
				},
			}
			thePod = &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "not-feeling-well",
					Annotations: map[string]string{},
					Labels: map[string]string{
						k8s.LabelStagingGUID: "the-stage-guid",
					},
				},
				Spec: v1.PodSpec{
					Containers:     []v1.Container{{}},
					InitContainers: []v1.Container{},
				},
			}
			containerStatus = statusFailed()
			initContainerStatus = statusOK()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/stage/the-stage-guid/completed"),
					ghttp.VerifyJSON(`{
  					"task_guid": "the-stage-guid",
  					"failed": true,
						"failure_reason": "Container 'failing-container' in Pod 'not-feeling-well' failed: ErrImagePull",
  					"result": "",
  					"annotation": "{\"lifecycle\":\"\",\"completion_callback\":\"internal_cc_staging_endpoint.io/stage/build_completed\"}",
  					"created_at": 0
  				}`),
				))
		})

		JustBeforeEach(func() {
			thePod.Spec.Containers[0].Env = env
			thePod.Status = v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					containerStatus,
				},
				InitContainerStatuses: []v1.ContainerStatus{
					initContainerStatus,
				},
			}

			reporter.Report(thePod)
		})

		Context("When a container is failing", func() {

			It("should report the correct container failure reason to Eirini", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("should log container failures in staging pods", func() {
				Expect(logger.Logs()).To(HaveLen(1))

				logs := logger.Logs()
				Expect(logs).To(HaveLen(1))
				log := logs[0]
				Expect(log.Message).To(Equal("staging-reporter-test.staging pod failed"))
				Expect(log.Data).To(HaveKeyWithValue("error", "Container 'failing-container' in Pod 'not-feeling-well' failed: ErrImagePull"))
			})
		})

		Context("When init containers are failing", func() {
			BeforeEach(func() {
				containerStatus = statusOK()
				initContainerStatus = statusFailed()
			})

			It("should detect failing InitContainers", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("should log container failures in staging pods", func() {
				Expect(logger.Logs()).To(HaveLen(1))

				logs := logger.Logs()
				Expect(logs).To(HaveLen(1))
				log := logs[0]
				Expect(log.Message).To(Equal("staging-reporter-test.staging pod failed"))
				Expect(log.Data).To(HaveKeyWithValue("error", "Container 'failing-container' in Pod 'not-feeling-well' failed: ErrImagePull"))
			})
		})

		Context("When no containers are in failing state", func() {
			BeforeEach(func() {
				containerStatus = statusOK()
				initContainerStatus = statusOK()
			})

			It("should silently ignore happy containers", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the COMPLETION_CALLBACK env variable is missing", func() {
			BeforeEach(func() {
				env = []v1.EnvVar{
					{
						Name:  "EIRINI_ADDRESS",
						Value: server.URL(),
					},
				}
			})

			It("logs the error", func() {
				Expect(logger.Logs()).To(HaveLen(1))

				logs := logger.Logs()
				Expect(logs).To(HaveLen(1))
				log := logs[0]
				Expect(log.Message).To(Equal("staging-reporter-test.getting env variable 'COMPLETION_CALLBACK' failed"))
			})

			It("should not talk to Eirini", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the EIRINI_ADDRESS env variable is missing", func() {
			BeforeEach(func() {
				env = []v1.EnvVar{
					{
						Name:  "COMPLETION_CALLBACK",
						Value: "internal_cc_staging_endpoint.io/stage/build_completed",
					},
				}
			})

			It("logs the error", func() {
				Expect(logger.Logs()).To(HaveLen(1))

				logs := logger.Logs()
				Expect(logs).To(HaveLen(1))
				log := logs[0]
				Expect(log.Message).To(Equal("staging-reporter-test.getting env variable 'EIRINI_ADDRESS' failed"))
			})

			It("should not talk to Eirini", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})
	})
})

func statusFailed() v1.ContainerStatus {
	return v1.ContainerStatus{
		Name: "failing-container",
		State: v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Reason: "ErrImagePull",
			},
		},
	}
}

func statusOK() v1.ContainerStatus {
	return v1.ContainerStatus{
		Name: "starting-container",
		State: v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Reason: "PodInitializing",
			},
		},
	}
}
