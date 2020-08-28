package event

import (
	"context"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/lager"
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
	Generate(*corev1.Pod, lager.Logger) (events.CrashEvent, bool)
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

func (c *CrashReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := c.logger.Session("reconcile-pod-crash",
		lager.Data{
			"name":      request.NamespacedName.Name,
			"namespace": request.NamespacedName.Namespace,
		})

	pod := &corev1.Pod{}

	err := c.client.Get(context.Background(), request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("pod-not-found", lager.Data{"error": err})

			return reconcile.Result{}, nil
		}

		logger.Error("failed-to-get-pod", err)

		return reconcile.Result{}, err
	}

	logger.Info("fetched-pod", lager.Data{"pod": pod})

	event, send := c.eventGenerator.Generate(pod, c.logger)
	if !send {
		logger.Debug("not-sending-event")

		return reconcile.Result{}, nil
	}

	logger.Info("generated-event", lager.Data{"event": event})

	err = c.crashEmitter.Emit(event)
	if err != nil {
		logger.Error("failed-to-emit-event", err)

		return reconcile.Result{}, err
	}

	logger.Info("emitted-event")

	return reconcile.Result{}, nil
}
