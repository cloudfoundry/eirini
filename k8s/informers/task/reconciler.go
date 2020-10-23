package task

import (
	"context"
	"strconv"
	"time"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-multierror"
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
	Report(*corev1.Pod) error
}

type JobsClient interface {
	GetByGUID(guid string, includeCompleted bool) ([]batchv1.Job, error)
	SetLabel(job *batchv1.Job, key, value string) (*batchv1.Job, error)
}

type Deleter interface {
	Delete(guid string) (string, error)
}

type PodsClient interface {
	SetAnnotation(pod *corev1.Pod, key, value string) (*corev1.Pod, error)
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

func (r Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.Session("task-completion-reconciler", lager.Data{"namespace": request.Namespace, "pod-name": request.Name})

	pod := &corev1.Pod{}
	if err := r.runtimeClient.Get(context.Background(), request.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error("pod does not exist", err)

			return reconcile.Result{}, nil
		}

		logger.Error("failed to get pod", err)

		return reconcile.Result{}, err
	}

	if !r.taskContainerHasTerminated(logger, pod) {
		return reconcile.Result{}, nil
	}

	guid := pod.Labels[k8s.LabelGUID]
	logger = logger.WithData(lager.Data{"guid": guid})

	jobsForPods, err := r.jobs.GetByGUID(guid, true)
	if err != nil {
		logger.Error("failed to get related job by guid", err)

		return reconcile.Result{}, err
	}

	if len(jobsForPods) == 0 {
		logger.Debug("no jobs found for this pod")

		return reconcile.Result{}, nil
	}

	if err = r.reportIfRequired(pod); err != nil {
		logger.Error("completion-callback-failed", err, lager.Data{"tries": pod.Annotations[k8s.AnnotationOpiTaskCompletionReportCounter]})

		return reconcile.Result{}, err
	}

	if _, err = r.jobs.SetLabel(&jobsForPods[0], k8s.LabelTaskCompleted, k8s.TaskCompletedTrue); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to label the job as completed")
	}

	if !r.taskHasExpired(logger, pod) {
		logger.Debug("task-hasnt-expired-yet")

		return reconcile.Result{RequeueAfter: time.Duration(r.ttlSeconds) * time.Second}, nil
	}

	_, err = r.deleter.Delete(guid)

	return reconcile.Result{}, err
}

func (r *Reconciler) reportIfRequired(pod *corev1.Pod) error {
	if pod.Annotations[k8s.AnnotationCCAckedTaskCompletion] == k8s.TaskCompletedTrue {
		return nil
	}

	completionCounter := parseIntOrZero(pod.Annotations[k8s.AnnotationOpiTaskCompletionReportCounter])
	if completionCounter >= r.callbackRetryLimit {
		return nil
	}

	if err := r.reporter.Report(pod); err != nil {
		resultErr := multierror.Append(err)

		if _, updateErr := r.pods.SetAnnotation(pod, k8s.AnnotationOpiTaskCompletionReportCounter, strconv.Itoa(completionCounter+1)); updateErr != nil {
			resultErr = multierror.Append(resultErr, updateErr)
		}

		return resultErr.ErrorOrNil()
	}

	if _, updateErr := r.pods.SetAnnotation(pod, k8s.AnnotationCCAckedTaskCompletion, k8s.TaskCompletedTrue); updateErr != nil {
		return updateErr
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

	return status.State.Terminated.FinishedAt.Time.Before(ttlExpire)
}

func parseIntOrZero(s string) int {
	value, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return value
}
