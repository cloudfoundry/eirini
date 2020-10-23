package task_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/k8s/informers/task/taskfakes"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Task Completion Reconciler", func() {
	var (
		reconcileRes  reconcile.Result
		reconcileErr  error
		logger        *lagertest.TestLogger
		runtimeClient *reconcilerfakes.FakeClient
		jobsClient    *taskfakes.FakeJobsClient
		podsClient    *taskfakes.FakePodsClient
		taskReporter  *taskfakes.FakeReporter
		taskDeleter   *taskfakes.FakeDeleter
		reconciler    *task.Reconciler
		pod           *corev1.Pod
		job           batchv1.Job
		ttl           int
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("reconciler-test")
		runtimeClient = new(reconcilerfakes.FakeClient)
		jobsClient = new(taskfakes.FakeJobsClient)
		podsClient = new(taskfakes.FakePodsClient)
		taskReporter = new(taskfakes.FakeReporter)
		taskDeleter = new(taskfakes.FakeDeleter)
		ttl = 60
		reconciler = task.NewReconciler(logger, runtimeClient, jobsClient, podsClient, taskReporter, taskDeleter, 2, ttl)

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					k8s.LabelGUID: "the-task-pod-guid",
				},
				Annotations: map[string]string{
					k8s.AnnotationOpiTaskContainerName: "opi-task",
				},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "opi-task",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:   0,
								FinishedAt: metav1.NewTime(time.Now().Add(-120 * time.Second)),
							},
						},
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

		runtimeClient.GetStub = func(c context.Context, nn k8stypes.NamespacedName, o runtime.Object) error {
			p := o.(*corev1.Pod)
			pod.DeepCopyInto(p)
			pod = p

			p.Labels[k8s.LabelGUID] = nn.Name + "-guid"

			return nil
		}

		job = batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{},
			},
		}
		jobsClient.GetByGUIDReturns([]batchv1.Job{job}, nil)
	})

	JustBeforeEach(func() {
		reconcileRes, reconcileErr = reconciler.Reconcile(reconcile.Request{
			NamespacedName: k8stypes.NamespacedName{
				Name:      "the-task-pod",
				Namespace: "space",
			},
		})
	})

	It("succeeds", func() {
		Expect(reconcileErr).NotTo(HaveOccurred())
	})

	It("fetches the task pod", func() {
		Expect(runtimeClient.GetCallCount()).To(Equal(1))
		_, actualNamepspacedName, _ := runtimeClient.GetArgsForCall(0)
		Expect(actualNamepspacedName.Namespace).To(Equal("space"))
		Expect(actualNamepspacedName.Name).To(Equal("the-task-pod"))
	})

	It("fetches the job by guid", func() {
		Expect(jobsClient.GetByGUIDCallCount()).To(Equal(1))
		actualGUID, actualIncludeCompleted := jobsClient.GetByGUIDArgsForCall(0)
		Expect(actualGUID).To(Equal("the-task-pod-guid"))
		Expect(actualIncludeCompleted).To(BeTrue())
	})

	It("reports the task pod", func() {
		Expect(taskReporter.ReportCallCount()).To(Equal(1))
		Expect(taskReporter.ReportArgsForCall(0).Name).To(Equal(pod.Name))
		Expect(podsClient.SetAnnotationCallCount()).To(Equal(1))
		actualPod, key, value := podsClient.SetAnnotationArgsForCall(0)
		Expect(actualPod).To(Equal(pod))
		Expect(key).To(Equal(k8s.AnnotationCCAckedTaskCompletion))
		Expect(value).To(Equal(k8s.TaskCompletedTrue))
	})

	It("deletes the task", func() {
		Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
		Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-pod-guid"))
	})

	It("labels the task as completed", func() {
		Expect(jobsClient.SetLabelCallCount()).To(Equal(1))
		_, label, value := jobsClient.SetLabelArgsForCall(0)
		Expect(label).To(Equal(k8s.LabelTaskCompleted))
		Expect(value).To(Equal(k8s.TaskCompletedTrue))
	})

	When("TTL has not yet expired", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(time.Now())
		})

		It("notifies CC, but does not delete yet", func() {
			Expect(taskReporter.ReportCallCount()).To(Equal(1))
			Expect(taskReporter.ReportArgsForCall(0).Name).To(Equal(pod.Name))

			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))

			Expect(reconcileErr).ToNot(HaveOccurred())
			Expect(reconcileRes.RequeueAfter).To(Equal(time.Second * time.Duration(ttl)))
		})
	})

	When("CC has been notified and TTL has expired", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(time.Now().Add(-60 * time.Second))
			pod.ObjectMeta.Annotations[k8s.AnnotationCCAckedTaskCompletion] = k8s.TaskCompletedTrue
		})

		It("deletes the job", func() {
			Expect(taskReporter.ReportCallCount()).To(Equal(0))
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			Expect(taskDeleter.DeleteArgsForCall(0)).To(Equal("the-task-pod-guid"))
			Expect(reconcileErr).ToNot(HaveOccurred())
			Expect(reconcileRes.IsZero()).To(BeTrue())
		})
	})

	When("fetching the task pod fails", func() {
		BeforeEach(func() {
			runtimeClient.GetReturns(errors.New("fetch-pod-error"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError("fetch-pod-error"))
		})

		It("does not call the task reporter", func() {
			Expect(taskReporter.ReportCallCount()).To(BeZero())
		})

		It("does not delete the task", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("task container has not completed", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses[0].State.Terminated = nil
			pod.Status.ContainerStatuses[0].State.Running = &corev1.ContainerStateRunning{}
		})

		It("exits immediately doing nothing", func() {
			Expect(reconcileErr).To(BeNil())
			Expect(jobsClient.GetByGUIDCallCount()).To(BeZero())
			Expect(taskReporter.ReportCallCount()).To(BeZero())
			Expect(taskDeleter.DeleteCallCount()).To(BeZero())
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

		It("exits immediately doing nothing", func() {
			Expect(reconcileErr).To(BeNil())
			Expect(jobsClient.GetByGUIDCallCount()).To(BeZero())
			Expect(taskReporter.ReportCallCount()).To(BeZero())
			Expect(taskDeleter.DeleteCallCount()).To(BeZero())
		})
	})

	When("fetching the job fails", func() {
		BeforeEach(func() {
			jobsClient.GetByGUIDReturns([]batchv1.Job{}, errors.New("fetch-job-failure"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError("fetch-job-failure"))
		})

		It("does not call the task reporter", func() {
			Expect(taskReporter.ReportCallCount()).To(BeZero())
		})

		It("does not delete the task", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("when the job for the pod no longer exists (because it has been deleted during a previous reconciliation event)", func() {
		BeforeEach(func() {
			jobsClient.GetByGUIDReturns([]batchv1.Job{}, nil)
		})

		It("does not return an error", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
		})

		It("does not call the task reporter", func() {
			Expect(taskReporter.ReportCallCount()).To(BeZero())
		})

		It("does not delete the task", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("the task pod does not exist", func() {
		BeforeEach(func() {
			runtimeClient.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, ""))
		})

		It("does not call the task reporter", func() {
			Expect(taskReporter.ReportCallCount()).To(BeZero())
		})

		It("does not delete the task", func() {
			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("reporting the task pod fails", func() {
		BeforeEach(func() {
			taskReporter.ReportReturns(errors.New("task-reporter-error"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("task-reporter-error")))
		})

		It("does not set the 'cc acked' annotation on the pod", func() {
			Expect(pod.Annotations[k8s.AnnotationCCAckedTaskCompletion]).To(BeEmpty())
		})

		It("updates the pod setting the updated call count but not reporting success", func() {
			Expect(podsClient.SetAnnotationCallCount()).To(Equal(1))
			actualPod, key, value := podsClient.SetAnnotationArgsForCall(0)
			Expect(actualPod).To(Equal(pod))
			Expect(key).To(Equal(k8s.AnnotationOpiTaskCompletionReportCounter))
			Expect(value).To(Equal("1"))
		})

		It("does not label the task as completed", func() {
			Expect(jobsClient.SetLabelCallCount()).To(BeZero())
		})

		When("it's the first time", func() {
			It("sets the 'retry counter' annotation", func() {
				Expect(podsClient.SetAnnotationCallCount()).To(Equal(1))
				actualPod, key, value := podsClient.SetAnnotationArgsForCall(0)
				Expect(actualPod).To(Equal(pod))
				Expect(key).To(Equal(k8s.AnnotationOpiTaskCompletionReportCounter))
				Expect(value).To(Equal("1"))
			})

			It("does not delete the task", func() {
				Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
			})
		})

		When("it's a subsequent time within the retry limit", func() {
			BeforeEach(func() {
				pod.ObjectMeta.Annotations[k8s.AnnotationOpiTaskCompletionReportCounter] = "1"
			})

			It("increments the reporting count", func() {
				Expect(podsClient.SetAnnotationCallCount()).To(Equal(1))
				actualPod, key, value := podsClient.SetAnnotationArgsForCall(0)
				Expect(actualPod).To(Equal(pod))
				Expect(key).To(Equal(k8s.AnnotationOpiTaskCompletionReportCounter))
				Expect(value).To(Equal("2"))
			})

			It("does not delete the task", func() {
				Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
			})
		})

		When("it hits the retry limit", func() {
			BeforeEach(func() {
				pod.ObjectMeta.Annotations[k8s.AnnotationOpiTaskCompletionReportCounter] = "2"
			})

			It("does not retry any more", func() {
				Expect(reconcileRes.IsZero()).To(BeTrue())
				Expect(reconcileErr).To(BeNil())
			})

			It("deletes the task", func() {
				Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			})
		})

		When("updating the annotation on the pod fails", func() {
			BeforeEach(func() {
				podsClient.SetAnnotationReturns(nil, errors.New("update-failed"))
			})

			It("returns an error with both failure messages", func() {
				Expect(reconcileErr).To(MatchError(SatisfyAll(
					ContainSubstring("task-reporter-error"),
					ContainSubstring("update-failed"),
				)))
			})
		})
	})

	When("deleting the job fails", func() {
		BeforeEach(func() {
			taskDeleter.DeleteReturns("", errors.New("delete-task-failure"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError("delete-task-failure"))
		})
	})

	When("labeling the job as completed fails", func() {
		BeforeEach(func() {
			jobsClient.SetLabelReturns(nil, errors.New("boom"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError("failed to label the job as completed: boom"))
		})
	})
})
