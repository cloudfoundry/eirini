package reconciler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	eiriniControllerSource = "eirini-controller"
	crashEventType         = "Warning"
	labelInstanceIndex     = "cloudfoundry.org/instance_index"
	action                 = "crashing"
	lrpKind                = "LRP"
	statefulSetKind        = "StatefulSet"
)

//counterfeiter:generate . CrashEventGenerator

type CrashEventGenerator interface {
	Generate(*corev1.Pod, lager.Logger) (events.CrashEvent, bool)
}

//counterfeiter:generate . EventsClient

type EventsClient interface {
	Create(namespace string, event *corev1.Event) (*corev1.Event, error)
	Update(namespace string, event *corev1.Event) (*corev1.Event, error)
	List(metav1.ListOptions) (*corev1.EventList, error)
}

type PodCrash struct {
	logger              lager.Logger
	pods                client.Client
	crashEventGenerator CrashEventGenerator
	eventsClient        EventsClient
	statefulSetGetter   StatefulSetGetter
}

func NewPodCrash(
	logger lager.Logger, client client.Client, crashEventGenerator CrashEventGenerator,
	eventsClient EventsClient, statefulSetGetter StatefulSetGetter) *PodCrash {
	return &PodCrash{
		logger:              logger,
		pods:                client,
		crashEventGenerator: crashEventGenerator,
		eventsClient:        eventsClient,
		statefulSetGetter:   statefulSetGetter,
	}
}

func (r PodCrash) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.Session("crash-event-reconciler", lager.Data{"namespace": request.Namespace, "name": request.Name})

	pod := &corev1.Pod{}
	if err := r.pods.Get(context.Background(), request.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error("pod does not exist", err)

			return reconcile.Result{}, nil
		}

		logger.Error("failed to get pod", err)

		return reconcile.Result{}, err
	}

	crashEvent, shouldCreate := r.crashEventGenerator.Generate(pod, logger)
	if !shouldCreate {
		return reconcile.Result{}, nil
	}

	statefulSetRef, err := r.getOwner(pod, statefulSetKind)
	if err != nil {
		logger.Debug("pod-without-statefulset-owner")

		return reconcile.Result{}, nil
	}

	statefulSet, err := r.statefulSetGetter.Get(pod.Namespace, statefulSetRef.Name)
	if err != nil {
		logger.Error("failed-to-get-stateful-set", err)

		return reconcile.Result{}, err
	}

	lrpRef, err := r.getOwner(statefulSet, lrpKind)
	if err != nil {
		logger.Debug("statefulset-without-lrp-owner", lager.Data{"statefulset-name": statefulSet.Name})

		return reconcile.Result{}, nil
	}

	kubeEvent, exists, err := r.getExistingEvent(lrpRef, request.Namespace, crashEvent)
	if err != nil {
		logger.Error("failed-to-get-existing-event", err)

		return reconcile.Result{}, err
	}

	if exists {
		return r.updateEvent(logger, kubeEvent, crashEvent, request.Namespace)
	}

	return r.createEvent(logger, lrpRef, crashEvent, request.Namespace)
}

func (r PodCrash) getExistingEvent(ownerRef metav1.OwnerReference, namespace string, crashEvent events.CrashEvent) (*corev1.Event, bool, error) {
	fieldSelector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s,involvedObject.namespace=%s,reason=%s",
		ownerRef.Kind,
		ownerRef.Name,
		namespace,
		failureReason(crashEvent))
	labelSelector := fmt.Sprintf("cloudfoundry.org/instance_index=%d", crashEvent.Index)

	kubeEvents, err := r.eventsClient.List(metav1.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list events")
	}

	if len(kubeEvents.Items) == 1 {
		return &kubeEvents.Items[0], true, nil
	}

	return nil, false, nil
}

func (r PodCrash) createEvent(logger lager.Logger, ownerRef metav1.OwnerReference, crashEvent events.CrashEvent, namespace string) (reconcile.Result, error) {
	var err error

	event := r.makeEvent(crashEvent, namespace, ownerRef)
	if event, err = r.eventsClient.Create(namespace, event); err != nil {
		logger.Error("failed-to-create-event", err)

		return reconcile.Result{}, errors.Wrap(err, "failed to create event")
	}

	logger.Debug("event-created", lager.Data{"name": event.Name, "namespace": event.Namespace})

	return reconcile.Result{}, nil
}

func (r PodCrash) updateEvent(logger lager.Logger, kubeEvent *corev1.Event, crashEvent events.CrashEvent, namespace string) (reconcile.Result, error) {
	kubeEvent.Count++
	kubeEvent.LastTimestamp = metav1.NewTime(time.Unix(crashEvent.CrashTimestamp, 0))

	if _, err := r.eventsClient.Update(namespace, kubeEvent); err != nil {
		logger.Error("failed-to-update-event", err)

		return reconcile.Result{}, errors.Wrap(err, "failed to update event")
	}

	logger.Debug("event-updated", lager.Data{"name": kubeEvent.Name, "namespace": kubeEvent.Namespace})

	return reconcile.Result{}, nil
}

func (r PodCrash) makeEvent(crashEvent events.CrashEvent, namespace string, involvedObjRef metav1.OwnerReference) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: fmt.Sprintf("%s-", crashEvent.Instance),
			Labels: map[string]string{
				labelInstanceIndex: strconv.Itoa(crashEvent.Index),
			},
			Annotations: map[string]string{
				k8s.AnnotationProcessGUID: crashEvent.ProcessGUID,
			},
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       involvedObjRef.Kind,
			Name:       involvedObjRef.Name,
			UID:        involvedObjRef.UID,
			APIVersion: involvedObjRef.APIVersion,
			Namespace:  namespace,
			FieldPath:  "spec.containers{opi}",
		},
		Reason:  failureReason(crashEvent),
		Message: fmt.Sprintf("Container terminated with exit code: %d", crashEvent.ExitStatus),
		Source: corev1.EventSource{
			Component: eiriniControllerSource,
		},
		FirstTimestamp:      metav1.NewTime(time.Unix(crashEvent.CrashTimestamp, 0)),
		LastTimestamp:       metav1.NewTime(time.Unix(crashEvent.CrashTimestamp, 0)),
		EventTime:           metav1.NewMicroTime(time.Now()),
		Count:               1,
		Type:                crashEventType,
		ReportingController: eiriniControllerSource,
		Action:              action,
		ReportingInstance:   "controller-id",
	}
}

func (r PodCrash) getOwner(obj metav1.Object, kind string) (metav1.OwnerReference, error) {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == kind {
			return ref, nil
		}
	}

	return metav1.OwnerReference{}, fmt.Errorf("no owner of kind %q", kind)
}

func failureReason(crashEvent events.CrashEvent) string {
	return fmt.Sprintf("Container: %s", crashEvent.Reason)
}
