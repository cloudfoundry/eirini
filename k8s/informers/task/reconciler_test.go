package task_test

import (
	"context"
	"errors"

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
		reconcileErr error
		logger       *lagertest.TestLogger
		podClient    *reconcilerfakes.FakeClient
		jobsClient   *taskfakes.FakeJobsClient
		taskReporter *taskfakes.FakeReporter
		reconciler   *task.Reconciler
		pod          *corev1.Pod
		job          batchv1.Job
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("reconciler-test")
		podClient = new(reconcilerfakes.FakeClient)
		jobsClient = new(taskfakes.FakeJobsClient)
		taskReporter = new(taskfakes.FakeReporter)
		reconciler = task.NewReconciler(logger, podClient, jobsClient, taskReporter)

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					k8s.LabelGUID: "the-guid",
				},
			},
		}

		podClient.GetStub = func(c context.Context, nn k8stypes.NamespacedName, o runtime.Object) error {
			p := o.(*corev1.Pod)
			p.Name = pod.Name
			p.Namespace = pod.Namespace
			p.Labels = pod.Labels

			return nil
		}

		jobsClient.GetByGUIDReturns([]batchv1.Job{job}, nil)
	})

	JustBeforeEach(func() {
		_, reconcileErr = reconciler.Reconcile(reconcile.Request{
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
		Expect(podClient.GetCallCount()).To(Equal(1))
		_, actualNamepspacedName, _ := podClient.GetArgsForCall(0)
		Expect(actualNamepspacedName.Namespace).To(Equal("space"))
		Expect(actualNamepspacedName.Name).To(Equal("the-task-pod"))
	})

	It("fetches the job by guid", func() {
		Expect(jobsClient.GetByGUIDCallCount()).To(Equal(1))
		actualGUID := jobsClient.GetByGUIDArgsForCall(0)
		Expect(actualGUID).To(Equal("the-guid"))
	})

	It("reports the task pod", func() {
		Expect(taskReporter.ReportCallCount()).To(Equal(1))
		Expect(taskReporter.ReportArgsForCall(0)).To(Equal(pod))
	})

	When("fetching the task pod fails", func() {
		BeforeEach(func() {
			podClient.GetReturns(errors.New("fetch-pod-error"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError("fetch-pod-error"))
		})

		It("does not call the task reporter", func() {
			Expect(taskReporter.ReportCallCount()).To(BeZero())
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
	})

	When("the task pod does not exist", func() {
		BeforeEach(func() {
			podClient.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, ""))
		})

		It("does not call the task reporter", func() {
			Expect(taskReporter.ReportCallCount()).To(BeZero())
		})
	})

	When("reporting the task pod fails", func() {
		BeforeEach(func() {
			taskReporter.ReportReturns(errors.New("task-reporter-error"))
		})

		It("returns the error", func() {
			Expect(reconcileErr).To(MatchError("task-reporter-error"))
		})
	})
})
