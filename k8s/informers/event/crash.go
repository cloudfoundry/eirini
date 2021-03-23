package event

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CrashLoopBackOff           = "CrashLoopBackOff"
	CreateContainerConfigError = "CreateContainerConfigError"
)

//counterfeiter:generate . CrashEventGenerator
//counterfeiter:generate . CrashEmitter

type CrashEventGenerator interface {
	Generate(context.Context, *corev1.Pod, lager.Logger) (events.CrashEvent, bool)
}

type CrashEmitter interface {
	Emit(events.CrashEvent) error
}

type CrashReconciler struct {
	logger         lager.Logger
	client         client.Client
	eventGenerator CrashEventGenerator
	crashEmitter   CrashEmitter
}

func NewCrashReconciler(
	logger lager.Logger,
	client client.Client,
	eventGenerator CrashEventGenerator,
	crashEmitter CrashEmitter,
) *CrashReconciler {
	return &CrashReconciler{
		logger:         logger,
		client:         client,
		eventGenerator: eventGenerator,
		crashEmitter:   crashEmitter,
	}
}

func (c *CrashReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := c.logger.Session("reconcile-pod-crash",
		lager.Data{
			"name":      request.NamespacedName.Name,
			"namespace": request.NamespacedName.Namespace,
		})

	pod := &corev1.Pod{}

	err := c.client.Get(ctx, request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("pod-not-found", lager.Data{"error": err})

			return reconcile.Result{}, nil
		}

		logger.Error("failed-to-get-pod", err)

		return reconcile.Result{}, errors.Wrap(err, "failed to get pod")
	}

	event, send := c.eventGenerator.Generate(ctx, pod, c.logger)
	if !send {
		logger.Debug("not-sending-event")

		return reconcile.Result{}, nil
	}

	if strconv.FormatInt(event.CrashTimestamp, 10) == pod.Annotations[stset.AnnotationLastReportedAppCrash] {
		logger.Debug("event-already-sent")

		return reconcile.Result{}, nil
	}

	logger.Info("generated-event", lager.Data{"event": event})

	err = c.crashEmitter.Emit(event)
	if err != nil {
		logger.Error("failed-to-emit-event", err)

		return reconcile.Result{}, errors.Wrap(err, "failed to emit event")
	}

	logger.Info("emitted-event")

	newPod := pod.DeepCopy()
	if newPod.Annotations == nil {
		newPod.Annotations = map[string]string{}
	}

	newPod.Annotations[stset.AnnotationLastReportedAppCrash] = strconv.FormatInt(event.CrashTimestamp, 10)

	if err = c.client.Patch(ctx, newPod, client.MergeFrom(pod)); err != nil {
		logger.Error("failed-to-set-last-crash-time-on-pod", err)
	}

	return reconcile.Result{}, nil
}
