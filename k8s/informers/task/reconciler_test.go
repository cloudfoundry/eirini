package task_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/eirini/k8s/informers/event/eventfakes"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/k8s/informers/task/taskfakes"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Task Completion Reconciler", func() {
	var (
		reconcileRes  reconcile.Result
		reconcileErr  error
		logger        *lagertest.TestLogger
		runtimeClient *eventfakes.FakeClient
		jobsClient    *taskfakes.FakeJobsClient
		podsClient    *taskfakes.FakePodsClient
		taskReporter  *taskfakes.FakeReporter
		taskDeleter   *taskfakes.FakeDeleter
		reconciler    *task.Reconciler
		pod           *corev1.Pod
		jobslice      []batchv1.Job
		getByGUIDErr  error
		ttl           int
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("reconciler-test")
		runtimeClient = new(eventfakes.FakeClient)
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
					jobs.LabelGUID: "the-task-pod-guid",
				},
				Annotations: map[string]string{
					jobs.AnnotationTaskContainerName: "opi-task",
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

		runtimeClient.GetStub = func(c context.Context, nn k8stypes.NamespacedName, o client.Object) error {
			p, ok := o.(*corev1.Pod)
			Expect(ok).To(BeTrue())
			pod.DeepCopyInto(p)
			pod = p

			p.Labels[jobs.LabelGUID] = nn.Name + "-guid"

			return nil
		}

		jobslice = []batchv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
		}
		getByGUIDErr = nil
	})

	JustBeforeEach(func() {
		jobsClient.GetByGUIDReturns(jobslice, getByGUIDErr)

		reconcileRes, reconcileErr = reconciler.Reconcile(context.Background(), reconcile.Request{
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
		_, actualGUID, actualIncludeCompleted := jobsClient.GetByGUIDArgsForCall(0)
		Expect(actualGUID).To(Equal("the-task-pod-guid"))
		Expect(actualIncludeCompleted).To(BeTrue())
	})

	It("reports the task pod", func() {
		Expect(taskReporter.ReportCallCount()).To(Equal(1))
		_, actualPod := taskReporter.ReportArgsForCall(0)
		Expect(actualPod.Name).To(Equal(pod.Name))
		Expect(podsClient.SetAndTestAnnotationCallCount()).To(Equal(1))
		Expect(podsClient.SetAnnotationCallCount()).To(Equal(1))

		_, actualPod, key, value, prevValue := podsClient.SetAndTestAnnotationArgsForCall(0)
		Expect(actualPod).To(Equal(pod))
		Expect(key).To(Equal(jobs.AnnotationTaskCompletionReportCounter))
		Expect(value).To(Equal("1"))
		Expect(prevValue).To(BeNil())

		_, actualPod, key, value = podsClient.SetAnnotationArgsForCall(0)
		Expect(actualPod).To(Equal(pod))
		Expect(key).To(Equal(jobs.AnnotationCCAckedTaskCompletion))
		Expect(value).To(Equal(jobs.TaskCompletedTrue))
	})

	It("deletes the task", func() {
		Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
		_, actualPodGUID := taskDeleter.DeleteArgsForCall(0)
		Expect(actualPodGUID).To(Equal("the-task-pod-guid"))
	})

	It("labels the task as completed", func() {
		Expect(jobsClient.SetLabelCallCount()).To(Equal(1))
		_, _, label, value := jobsClient.SetLabelArgsForCall(0)
		Expect(label).To(Equal(jobs.LabelTaskCompleted))
		Expect(value).To(Equal(jobs.TaskCompletedTrue))
	})

	When("TTL has not yet expired", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(time.Now())
		})

		It("notifies CC, but does not delete yet", func() {
			Expect(taskReporter.ReportCallCount()).To(Equal(1))
			_, actualPod := taskReporter.ReportArgsForCall(0)
			Expect(actualPod.Name).To(Equal(pod.Name))

			Expect(taskDeleter.DeleteCallCount()).To(Equal(0))

			Expect(reconcileErr).ToNot(HaveOccurred())
			Expect(reconcileRes.RequeueAfter).To(Equal(time.Second * time.Duration(ttl)))
		})
	})

	When("CC has been notified and TTL has expired", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(time.Now().Add(-60 * time.Second))
			pod.ObjectMeta.Annotations[jobs.AnnotationCCAckedTaskCompletion] = jobs.TaskCompletedTrue
		})

		It("deletes the job", func() {
			Expect(taskReporter.ReportCallCount()).To(Equal(0))
			Expect(taskDeleter.DeleteCallCount()).To(Equal(1))
			_, actualPodGUID := taskDeleter.DeleteArgsForCall(0)
			Expect(actualPodGUID).To(Equal("the-task-pod-guid"))
			Expect(reconcileErr).ToNot(HaveOccurred())
			Expect(reconcileRes.IsZero()).To(BeTrue())
		})
	})

	When("fetching the task pod fails", func() {
		BeforeEach(func() {
			runtimeClient.GetReturns(errors.New("fetch-pod-error"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("fetch-pod-error")))
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

	When("job is owned by a task cr", func() {
		BeforeEach(func() {
			jobslice[0].ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				{
					Kind: "Task",
				},
			}
		})

		It("exits immediately doing nothing", func() {
			Expect(reconcileErr).To(BeNil())
			Expect(taskReporter.ReportCallCount()).To(BeZero())
			Expect(taskDeleter.DeleteCallCount()).To(BeZero())
		})
	})

	When("fetching the job fails", func() {
		BeforeEach(func() {
			jobslice = []batchv1.Job{}
			getByGUIDErr = errors.New("fetch-job-failure")
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("fetch-job-failure")))
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
			jobslice = []batchv1.Job{}
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
			Expect(pod.Annotations[jobs.AnnotationCCAckedTaskCompletion]).To(BeEmpty())
		})

		It("updates the pod setting the updated call count but not reporting success", func() {
			Expect(podsClient.SetAndTestAnnotationCallCount()).To(Equal(1))
			_, actualPod, key, value, prevValue := podsClient.SetAndTestAnnotationArgsForCall(0)
			Expect(actualPod).To(Equal(pod))
			Expect(key).To(Equal(jobs.AnnotationTaskCompletionReportCounter))
			Expect(value).To(Equal("1"))
			Expect(prevValue).To(BeNil())
		})

		It("does not label the task as completed", func() {
			Expect(jobsClient.SetLabelCallCount()).To(BeZero())
		})

		When("it's the first time", func() {
			It("sets the 'retry counter' annotation", func() {
				Expect(podsClient.SetAndTestAnnotationCallCount()).To(Equal(1))
				_, actualPod, key, value, prevValue := podsClient.SetAndTestAnnotationArgsForCall(0)
				Expect(actualPod).To(Equal(pod))
				Expect(key).To(Equal(jobs.AnnotationTaskCompletionReportCounter))
				Expect(value).To(Equal("1"))
				Expect(prevValue).To(BeNil())
			})

			It("does not delete the task", func() {
				Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
			})
		})

		When("it's a subsequent time within the retry limit", func() {
			BeforeEach(func() {
				pod.ObjectMeta.Annotations[jobs.AnnotationTaskCompletionReportCounter] = "1"
			})

			It("increments the reporting count", func() {
				Expect(podsClient.SetAndTestAnnotationCallCount()).To(Equal(1))
				_, actualPod, key, value, prevValue := podsClient.SetAndTestAnnotationArgsForCall(0)
				Expect(actualPod).To(Equal(pod))
				Expect(key).To(Equal(jobs.AnnotationTaskCompletionReportCounter))
				Expect(value).To(Equal("2"))
				Expect(prevValue).To(PointTo(Equal("1")))
			})

			It("does not delete the task", func() {
				Expect(taskDeleter.DeleteCallCount()).To(Equal(0))
			})
		})

		When("it hits the retry limit", func() {
			BeforeEach(func() {
				pod.ObjectMeta.Annotations[jobs.AnnotationTaskCompletionReportCounter] = "2"
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
				podsClient.SetAndTestAnnotationReturns(nil, errors.New("update-failed"))
			})

			It("doesn't attempt to contact CC", func() {
				Expect(taskReporter.ReportCallCount()).To(BeZero())
			})

			It("returns an error with both failure messages", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("update-failed")))
			})
		})
	})

	When("deleting the job fails", func() {
		BeforeEach(func() {
			taskDeleter.DeleteReturns("", errors.New("delete-task-failure"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("delete-task-failure")))
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
