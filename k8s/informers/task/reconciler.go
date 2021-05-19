package task

import (
	"context"
	"strconv"
	"time"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//counterfeiter:generate . Reporter
//counterfeiter:generate . JobsClient
//counterfeiter:generate . PodsClient
//counterfeiter:generate . Deleter

type Reporter interface {
	Report(context.Context, *corev1.Pod) error
}

type JobsClient interface {
	GetByGUID(ctx context.Context, guid string, includeCompleted bool) ([]batchv1.Job, error)
	SetLabel(ctx context.Context, job *batchv1.Job, key, value string) (*batchv1.Job, error)
}

type Deleter interface {
	Delete(ctx context.Context, guid string) (string, error)
}

type PodsClient interface {
	SetAnnotation(ctx context.Context, pod *corev1.Pod, key, value string) (*corev1.Pod, error)
	SetAndTestAnnotation(ctx context.Context, pod *corev1.Pod, key, value string, oldValue *string) (*corev1.Pod, error)
}

type Reconciler struct {
	logger             lager.Logger
	runtimeClient      client.Client
	jobs               JobsClient
	pods               PodsClient
	reporter           Reporter
	deleter            Deleter
	callbackRetryLimit int
	ttlSeconds         int
}

func NewReconciler(
	logger lager.Logger,
	podClient client.Client,
	jobsClient JobsClient,
	podUpdater PodsClient,
	reporter Reporter,
	deleter Deleter,
	callbackRetryLimit int,
	ttlSeconds int,
) *Reconciler {
	return &Reconciler{
		logger:             logger,
		runtimeClient:      podClient,
		jobs:               jobsClient,
		pods:               podUpdater,
		reporter:           reporter,
		deleter:            deleter,
		callbackRetryLimit: callbackRetryLimit,
		ttlSeconds:         ttlSeconds,
	}
}

func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.Session("task-completion-reconciler", lager.Data{"namespace": request.Namespace, "pod-name": request.Name})

	pod := &corev1.Pod{}
	if err := r.runtimeClient.Get(ctx, request.NamespacedName, pod); err != nil {
		return handlePodGetError(logger, err)
	}

	if !r.taskContainerHasTerminated(logger, pod) {
		return reconcile.Result{}, nil
	}

	guid := pod.Labels[jobs.LabelGUID]
	logger = logger.WithData(lager.Data{"guid": guid})

	jobsForPods, err := r.jobs.GetByGUID(ctx, guid, true)
	if err != nil {
		logger.Error("failed to get related job by guid", err)

		return reconcile.Result{}, errors.Wrap(err, "failed to get related job by guid")
	}

	if len(jobsForPods) == 0 {
		logger.Debug("no jobs found for this pod")

		return reconcile.Result{}, nil
	}

	job := jobsForPods[0]

	if jobOwnedByTask(job) {
		logger.Debug("ignoring job owned by a Task CR")

		return reconcile.Result{}, nil
	}

	if err = r.reportIfRequired(ctx, pod); err != nil {
		logger.Error("completion-callback-failed", err, lager.Data{"tries": pod.Annotations[jobs.AnnotationTaskCompletionReportCounter]})

		return reconcile.Result{}, err
	}

	if _, err = r.jobs.SetLabel(ctx, &job, jobs.LabelTaskCompleted, jobs.TaskCompletedTrue); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to label the job as completed")
	}

	if !r.taskHasExpired(logger, pod) {
		logger.Debug("task-hasnt-expired-yet")

		return reconcile.Result{RequeueAfter: time.Duration(r.ttlSeconds) * time.Second}, nil
	}

	logger.Debug("deleting-task")

	if _, err = r.deleter.Delete(ctx, guid); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to delete job")
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) reportIfRequired(ctx context.Context, pod *corev1.Pod) error {
	if pod.Annotations[jobs.AnnotationCCAckedTaskCompletion] == jobs.TaskCompletedTrue {
		return nil
	}

	completionCounterStr := pod.Annotations[jobs.AnnotationTaskCompletionReportCounter]

	completionCounter := parseIntOrZero(completionCounterStr)
	if completionCounter >= r.callbackRetryLimit {
		return nil
	}

	var counterPointer *string
	if completionCounterStr != "" {
		counterPointer = &completionCounterStr
	}

	_, err := r.pods.SetAndTestAnnotation(
		ctx,
		pod,
		jobs.AnnotationTaskCompletionReportCounter,
		strconv.Itoa(completionCounter+1),
		counterPointer,
	)
	if err != nil {
		return errors.Wrap(err, "failed to patch annotation on pod")
	}

	if err := r.reporter.Report(ctx, pod); err != nil {
		return errors.Wrap(err, "failed to report task completion to CC")
	}

	if _, updateErr := r.pods.SetAnnotation(ctx, pod, jobs.AnnotationCCAckedTaskCompletion, jobs.TaskCompletedTrue); updateErr != nil {
		return errors.Wrap(updateErr, "failed to set task completion annotation")
	}

	return nil
}

func (r Reconciler) taskContainerHasTerminated(logger lager.Logger, pod *corev1.Pod) bool {
	status, ok := getTaskContainerStatus(pod)
	if !ok {
		logger.Info("pod-has-no-task-container-status")

		return false
	}

	return status.State.Terminated != nil
}

func (r Reconciler) taskHasExpired(logger lager.Logger, pod *corev1.Pod) bool {
	status, ok := getTaskContainerStatus(pod)
	if !ok {
		logger.Info("pod-has-no-task-container-status")

		return false
	}

	ttlExpire := time.Now().Add(-time.Duration(r.ttlSeconds) * time.Second)

	logger.Debug("task-has-completed", lager.Data{"expiration-time": ttlExpire, "completion-time": status.State.Terminated.FinishedAt.Time})

	return status.State.Terminated.FinishedAt.Time.Before(ttlExpire)
}

func handlePodGetError(logger lager.Logger, err error) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) {
		logger.Error("pod does not exist", err)

		return reconcile.Result{}, nil
	}

	logger.Error("failed to get pod", err)

	return reconcile.Result{}, errors.Wrap(err, "failed to get pod")
}

func parseIntOrZero(s string) int {
	value, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return value
}

func jobOwnedByTask(job batchv1.Job) bool {
	for _, ref := range job.GetOwnerReferences() {
		if ref.Kind == "Task" {
			return true
		}
	}

	return false
}
