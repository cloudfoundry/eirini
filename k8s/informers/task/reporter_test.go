package task_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/k8s/informers/task/taskfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Reporter", func() {

	var (
		reporter    task.StateReporter
		server      *ghttp.Server
		logger      *lagertest.TestLogger
		pod         *corev1.Pod
		oldPod      *corev1.Pod
		taskDeleter *taskfakes.FakeDeleter
		handlers    []http.HandlerFunc
	)
	createPod := func(taskState corev1.ContainerState) *corev1.Pod {

		return &corev1.Pod{ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				k8s.LabelSourceType: "TASK",
			},
			Annotations: map[string]string{
				k8s.AnnotationOpiTaskContainerName: "opi-task",
				k8s.AnnotationGUID:                 "the-task-guid",
				k8s.AnnotationCompletionCallback:   fmt.Sprintf("%s/the-callback-url", server.URL()),
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
		taskDeleter = new(taskfakes.FakeDeleter)

		server = ghttp.NewServer()
		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("POST", "/the-callback-url"),
			ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
				TaskGUID: "the-task-guid",
			}),
		}

		reporter = task.StateReporter{
			Client:      &http.Client{},
			Logger:      logger,
			TaskDeleter: taskDeleter,
		}

		oldPod = createPod(corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{},
		})

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

		reporter.Report(oldPod, pod)
	})

	AfterEach(func() {
		server.Close()
	})

	It("notifies the cloud controller", func() {
		Expect(server.ReceivedRequests()).To(HaveLen(1))
	})

	It("deletes the job on kubernetes", func() {
		Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
		Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
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

		It("deletes the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
		})
	})

	When("task container has not completed", func() {
		BeforeEach(func() {
			pod = createPod(corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{},
			})
		})

		It("doesn't send anything to the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(0))
		})

		It("doesn't send delete the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("task container has already terminated", func() {
		BeforeEach(func() {
			oldPod = createPod(corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
				},
			})
		})

		It("doesn't send anything to the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(0))
		})

		It("doesn't send delete the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("there is no previous task container status", func() {
		BeforeEach(func() {
			oldPod = createPod(corev1.ContainerState{})
		})

		It("notifies the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		It("deletes the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
		})
	})

	When("task container status is missing", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: "some-sidecar",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			}
		})

		It("doesn't send anything to the cloud controller", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(0))
		})

		It("doesn't send delete the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
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

		It("logs the error", func() {
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("task-reporter-test.cannot send task status response"))
			Expect(log.Data).To(HaveKeyWithValue("error", "request not successful: status=502 potato"))
		})

		It("still deletes the job on kubernetes", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-guid"))
		})
	})
})
