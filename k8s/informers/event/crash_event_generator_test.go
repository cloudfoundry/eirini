package event_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
)

var crashTime = meta.Time{Time: time.Now()}

var _ = Describe("CrashEventGenerator", func() {
	var (
		clientset *fake.Clientset
		logger    *lagertest.TestLogger
		pod       *v1.Pod
		generator event.DefaultCrashEventGenerator
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("crash-event-logger-test")
		clientset = fake.NewSimpleClientset()
		eventsClient := client.NewEvent(clientset)
		generator = event.NewDefaultCrashEventGenerator(eventsClient)
	})

	Context("When app has been terminated", func() {
		Context("When there is one container in the pod", func() {
			BeforeEach(func() {
				pod = newTerminatedPod()
			})

			It("should generate a crashed report", func() {
				report, returned := generator.Generate(ctx, pod, logger)
				Expect(returned).To(BeTrue())
				Expect(report).To(Equal(events.CrashEvent{
					ProcessGUID: "test-pod-anno",
					AppCrashedRequest: cc_messages.AppCrashedRequest{
						Reason:          "better luck next time",
						Instance:        "test-pod-0",
						Index:           0,
						ExitStatus:      0,
						ExitDescription: "better luck next time",
						CrashCount:      9,
						CrashTimestamp:  crashTime.Time.Unix(),
					},
				}))
			})

			Context("When a pod is not owned by eirini", func() {
				BeforeEach(func() {
					pod.Labels = map[string]string{}
				})

				It("should not generate", func() {
					_, returned := generator.Generate(ctx, pod, logger)
					Expect(returned).To(BeFalse())
				})

				It("should provide a helpful log message", func() {
					generator.Generate(ctx, pod, logger)

					logs := logger.Logs()
					Expect(logs).To(HaveLen(1))
					log := logs[0]
					Expect(log.Message).To(Equal("crash-event-logger-test.generate-crash-event.skipping-non-eirini-pod"))
					Expect(log.Data).To(HaveKeyWithValue("pod-name", "test-pod-0"))
				})
			})

			Context("When pod is waiting, but hasn't been terminated", func() {
				BeforeEach(func() {
					pod = newPod([]v1.ContainerStatus{
						{
							Name:         stset.OPIContainerName,
							RestartCount: 0,
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{
									Reason: "better luck next time",
								},
							},
						},
					})
				})

				It("should not emit a crashed event", func() {
					_, returned := generator.Generate(ctx, pod, logger)
					Expect(returned).To(BeFalse())
				})
			})

			Context("When pod is stopped", func() {
				BeforeEach(func() {
					event := v1.Event{
						InvolvedObject: v1.ObjectReference{
							Namespace: "not-default",
							Name:      "pinky-pod",
						},
						Reason: "Killing",
					}
					_, clientErr := clientset.CoreV1().Events("not-default").Create(context.Background(), &event, meta.CreateOptions{})
					Expect(clientErr).ToNot(HaveOccurred())
				})

				It("should not emit a crashed event", func() {
					_, returned := generator.Generate(ctx, pod, logger)
					Expect(returned).To(BeFalse())
				})
			})

			Context("When pod is running", func() {
				BeforeEach(func() {
					pod = newRunningLastTerminatedPod()
				})

				It("sends a crash report", func() {
					_, returned := generator.Generate(ctx, pod, logger)
					Expect(returned).To(BeTrue())
				})
			})

			Context("When getting events fails", func() {
				BeforeEach(func() {
					reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("boom")
					}
					clientset.PrependReactor("list", "events", reaction)
				})

				It("should not emit a crashed event", func() {
					_, returned := generator.Generate(ctx, pod, logger)
					Expect(returned).To(BeFalse())
				})

				It("should provide a helpful log message", func() {
					generator.Generate(ctx, pod, logger)
					logs := logger.Logs()
					Expect(logs).To(HaveLen(1))
					log := logs[0]
					Expect(log.Message).To(Equal("crash-event-logger-test.generate-crash-event.skipping-failed-to-get-k8s-events"))
					Expect(log.Data).To(HaveKeyWithValue("pod-name", "test-pod-0"))
					Expect(log.Data).To(HaveKeyWithValue("guid", "test-pod-anno"))
					Expect(log.Data).To(HaveKeyWithValue("version", "test-pod-version"))
				})
			})
		})

		Context("When the sidecar is terminated", func() {
			BeforeEach(func() {
				pod = newTerminatedSidecarPod()
			})

			It("should not emit a crashed event", func() {
				_, returned := generator.Generate(ctx, pod, logger)
				Expect(returned).To(BeFalse())
			})
		})

		Context("When the sidecar is waiting after termination", func() {
			BeforeEach(func() {
				pod = newPod([]v1.ContainerStatus{
					{
						Name:         stset.OPIContainerName,
						RestartCount: 1,
						State: v1.ContainerState{
							Running: &v1.ContainerStateRunning{},
						},
					},
					{
						Name:         "some-sidecar-container",
						RestartCount: 1,
						State: v1.ContainerState{
							Waiting: &v1.ContainerStateWaiting{
								Reason:  event.CreateContainerConfigError,
								Message: "not configured properly",
							},
						},
						LastTerminationState: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Reason:     "better luck next time",
								FinishedAt: crashTime,
								ExitCode:   1,
							},
						},
					},
				})
			})

			It("should not emit a crashed event", func() {
				_, returned := generator.Generate(ctx, pod, logger)
				Expect(returned).To(BeFalse())
			})
		})
	})

	Context("When app is in waiting state after terimnation", func() {
		BeforeEach(func() {
			pod = newPod([]v1.ContainerStatus{
				{
					Name:         stset.OPIContainerName,
					RestartCount: 1,
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{
							Reason:  event.CreateContainerConfigError,
							Message: "not configured properly",
						},
					},
					LastTerminationState: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Reason:     "better luck next time",
							FinishedAt: crashTime,
							ExitCode:   1,
						},
					},
				},
			})
		})

		It("should return a crashed report", func() {
			report, returned := generator.Generate(ctx, pod, logger)
			Expect(returned).To(BeTrue())
			Expect(report).To(Equal(events.CrashEvent{
				ProcessGUID: "test-pod-anno",
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Reason:          "better luck next time",
					Instance:        "test-pod-0",
					ExitDescription: "better luck next time",
					ExitStatus:      1,
					CrashCount:      2,
					CrashTimestamp:  crashTime.Unix(),
				},
			}))
		})
	})

	Context("When a pod has no container statuses", func() {
		BeforeEach(func() {
			pod = newTerminatedPod()
		})

		Context("container statuses is nil", func() {
			BeforeEach(func() {
				pod.Status.ContainerStatuses = nil
			})

			It("should not send any reports", func() {
				_, returned := generator.Generate(ctx, pod, logger)
				Expect(returned).To(BeFalse())
			})
		})

		Context("container statuses is empty", func() {
			BeforeEach(func() {
				pod.Status.ContainerStatuses = []v1.ContainerStatus{}
			})

			It("should not send any reports", func() {
				_, returned := generator.Generate(ctx, pod, logger)
				Expect(returned).To(BeFalse())
			})
		})
	})

	Context("When a pod has no opi container statuses", func() {
		BeforeEach(func() {
			pod = newPod([]v1.ContainerStatus{
				{
					Name:         "some-other-container",
					RestartCount: 1,
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Reason:     "better luck next time",
							FinishedAt: crashTime,
							ExitCode:   1,
						},
					},
				},
			})
		})

		It("should not send any reports", func() {
			_, returned := generator.Generate(ctx, pod, logger)
			Expect(returned).To(BeFalse())
		})

		It("should provide a helpful log message", func() {
			generator.Generate(ctx, pod, logger)
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("crash-event-logger-test.generate-crash-event.skipping-eirini-pod-has-no-opi-container-statuses"))
			Expect(log.Data).To(HaveKeyWithValue("pod-name", "test-pod-0"))
			Expect(log.Data).To(HaveKeyWithValue("guid", "test-pod-anno"))
			Expect(log.Data).To(HaveKeyWithValue("version", "test-pod-version"))
		})
	})
})

func newTerminatedPod() *v1.Pod {
	return newPod([]v1.ContainerStatus{
		{
			Name:         "some-sidecar-container",
			RestartCount: 1,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{},
			},
		},
		{
			Name:         stset.OPIContainerName,
			RestartCount: 8,
			State: v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					Reason:     "better luck next time",
					FinishedAt: crashTime,
					ExitCode:   0,
				},
			},
		},
	})
}

func newRunningLastTerminatedPod() *v1.Pod {
	return newPod([]v1.ContainerStatus{
		{
			Name:         "some-sidecar-container",
			RestartCount: 1,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{},
			},
		},
		{
			Name:         stset.OPIContainerName,
			RestartCount: 8,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{},
			},
			LastTerminationState: v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					Reason:     "better luck next time",
					FinishedAt: crashTime,
					ExitCode:   0,
				},
			},
		},
	})
}

func newTerminatedSidecarPod() *v1.Pod {
	return newPod([]v1.ContainerStatus{
		{
			Name:         stset.OPIContainerName,
			RestartCount: 1,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{},
			},
		},
		{
			Name:         "some-sidecar-container",
			RestartCount: 8,
			State: v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					Reason:     "better luck next time",
					FinishedAt: crashTime,
					ExitCode:   0,
				},
			},
		},
	})
}

func newPod(statuses []v1.ContainerStatus) *v1.Pod {
	name := "test-pod"

	return &v1.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%s-%d", name, 0),
			Labels: map[string]string{
				stset.LabelSourceType: stset.AppSourceType,
			},
			Annotations: map[string]string{
				stset.AnnotationProcessGUID: fmt.Sprintf("%s-anno", name),
				stset.AnnotationVersion:     fmt.Sprintf("%s-version", name),
			},
			OwnerReferences: []meta.OwnerReference{
				{
					Kind: "StatefulSet",
					Name: "mr-stateful",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: statuses,
		},
	}
}
