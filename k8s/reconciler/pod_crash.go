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
)

//counterfeiter:generate . CrashEventGenerator

type CrashEventGenerator interface {
	Generate(*corev1.Pod, lager.Logger) (events.CrashEvent, bool)
}

//counterfeiter:generate . EventsClient

type EventsClient interface {
	Create(ctx context.Context, event *corev1.Event, opts metav1.CreateOptions) (*corev1.Event, error)
	// Update(ctx context.Context, event *corev1.Event, opts metav1.UpdateOptions) (*corev1.Event, error)
	// Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Event, error)
	// List(ctx context.Context, opts metav1.ListOptions) (*corev1.EventList, error)
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
	ctx := context.Background()
	if err := r.pods.Get(ctx, request.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Error("pod does not exist", err)
			return reconcile.Result{}, nil
		}
		r.logger.Error("failed to get pod", err)
		return reconcile.Result{}, err
	}

	crashEvent, shouldCreate := r.crashEventGenerator.Generate(pod, logger)

	if !shouldCreate {
		return reconcile.Result{}, nil
	}

	statefulSetRef, err := r.getOwner(pod, "StatefulSet")
	if err != nil {
		r.logger.Debug("pod-without-statefulset-owner")
		return reconcile.Result{}, nil
	}

	statefulSet, err := r.statefulSetGetter.Get(pod.Namespace, statefulSetRef.Name)
	if err != nil {
		r.logger.Error("failed-to-get-stateful-set", err)
		return reconcile.Result{}, err
	}

	lrpRef, err := r.getOwner(statefulSet, "LRP")
	if err != nil {
		r.logger.Debug("statefulset-without-lrp-owner", lager.Data{"statefulset-name": statefulSet.Name})
		return reconcile.Result{}, nil
	}

	event := r.makeEvent(crashEvent, pod.Namespace, lrpRef)
	if event, err = r.eventsClient.Create(ctx, event, metav1.CreateOptions{}); err != nil {
		r.logger.Error("failed-to-create-event", err)
		return reconcile.Result{}, errors.Wrap(err, "failed to create event")
	}
	r.logger.Debug("event-created", lager.Data{"name": event.Name, "namespace": event.Namespace})

	return reconcile.Result{}, nil
}

func (r PodCrash) makeEvent(crashEvent events.CrashEvent, namespace string, involvedObjRef metav1.OwnerReference) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: crashEvent.Instance,
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
		Reason:  crashEvent.Reason,
		Message: fmt.Sprintf("exit code: %d, message: %s", crashEvent.ExitStatus, crashEvent.ExitDescription),
		Source: corev1.EventSource{
			Component: eiriniControllerSource,
		},
		FirstTimestamp: metav1.Time{
			Time: time.Unix(crashEvent.CrashTimestamp, 0),
		},
		LastTimestamp: metav1.Time{
			Time: time.Unix(crashEvent.CrashTimestamp, 0),
		},
		EventTime: metav1.MicroTime{
			Time: time.Unix(crashEvent.CrashTimestamp, 0),
		},
		Count: 1,
		Type:  crashEventType,
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
