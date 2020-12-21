package task_test

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Reporter", func() {
	var (
		reporter task.StateReporter
		server   *ghttp.Server
		logger   *lagertest.TestLogger
		pod      *corev1.Pod
		handlers []http.HandlerFunc
		err      error
	)

	createPod := func(taskState corev1.ContainerState) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					jobs.LabelSourceType: "TASK",
				},
				Annotations: map[string]string{
					jobs.AnnotationOpiTaskContainerName: "opi-task",
					jobs.AnnotationGUID:                 "the-task-guid",
					jobs.AnnotationCompletionCallback:   fmt.Sprintf("%s/the-callback-url", server.URL()),
				},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:  "opi-task",
						State: taskState,
					},
					{
						Name: "some-sidecar",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					},
				},
			},
		}
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("task-reporter-test")

		server = ghttp.NewServer()
		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("POST", "/the-callback-url"),
			ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
				TaskGUID: "the-task-guid",
			}),
		}

		reporter = task.StateReporter{
			Client: &http.Client{},
			Logger: logger,
		}

		pod = createPod(corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
			},
		})
	})

	JustBeforeEach(func() {
		server.AppendHandlers(
			ghttp.CombineHandlers(handlers...),
		)

		err = reporter.Report(pod)
	})

	AfterEach(func() {
		server.Close()
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("notifies the cloud controller", func() {
		Expect(server.ReceivedRequests()).To(HaveLen(1))
	})

	When("the task container failed", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: "opi-task",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 42,
							Reason:   "because",
						},
					},
				},
				{
					Name: "some-sidecar",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			}

			handlers = []http.HandlerFunc{
				ghttp.VerifyRequest("POST", "/the-callback-url"),
				ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
					TaskGUID:      "the-task-guid",
					Failed:        true,
					FailureReason: "because",
				}),
			}
		})

		It("notifies the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})
	})

	When("the cloud controller returns an unexpected status code", func() {
		BeforeEach(func() {
			server.Reset()
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/the-callback-url"),
					ghttp.RespondWith(http.StatusBadGateway, "potato"),
				),
			)
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("status=502 potato")))
		})
	})
})
