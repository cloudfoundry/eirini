package event

import (
	"context"
	"encoding/json"

	"code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/models/cf"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . StatefulSetGetter

type StatefulSetGetter interface {
	Get(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error)
}

type PodUpdateHandler struct {
	StatefulSetGetter StatefulSetGetter
	Logger            lager.Logger
	RouteEmitter      eiriniroute.Emitter
}

func (h PodUpdateHandler) Handle(ctx context.Context, oldPod, updatedPod *corev1.Pod) {
	loggerSession := h.Logger.Session("pod-update", lager.Data{"pod-name": updatedPod.Name, "guid": updatedPod.Annotations[stset.AnnotationProcessGUID]})

	userDefinedRoutes, err := h.getUserDefinedRoutes(ctx, updatedPod)
	if err != nil {
		loggerSession.Debug("failed-to-get-user-defined-routes", lager.Data{"error": err.Error()})

		return
	}

	if markedForDeletion(*updatedPod) || (!isReady(updatedPod.Status.Conditions) && isReady(oldPod.Status.Conditions)) {
		loggerSession.Debug("pod-not-ready", lager.Data{"statuses": updatedPod.Status.Conditions, "deletion-timestamp": updatedPod.DeletionTimestamp})
		h.unregisterPodRoutes(ctx, oldPod, userDefinedRoutes)

		return
	}

	for _, r := range userDefinedRoutes {
		routes, err := route.NewRouteMessage(
			updatedPod,
			uint32(r.Port),
			eiriniroute.Routes{RegisteredRoutes: []string{r.Hostname}},
		)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})

			continue
		}

		h.RouteEmitter.Emit(ctx, *routes)
	}
}

func (h PodUpdateHandler) unregisterPodRoutes(ctx context.Context, pod *corev1.Pod, userDefinedRoutes []cf.Route) {
	loggerSession := h.Logger.Session("pod-delete", lager.Data{"pod-name": pod.Name, "guid": pod.Annotations[stset.AnnotationProcessGUID]})

	for _, r := range userDefinedRoutes {
		routes, err := route.NewRouteMessage(
			pod,
			uint32(r.Port),
			eiriniroute.Routes{UnregisteredRoutes: []string{r.Hostname}},
		)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})

			continue
		}

		h.RouteEmitter.Emit(ctx, *routes)
	}
}

func (h PodUpdateHandler) getUserDefinedRoutes(ctx context.Context, pod *corev1.Pod) ([]cf.Route, error) {
	owner, err := h.getOwner(ctx, pod)
	if err != nil {
		return []cf.Route{}, errors.Wrap(err, "failed to get owner")
	}

	return decodeRoutes(owner.Annotations[stset.AnnotationRegisteredRoutes])
}

func (h PodUpdateHandler) getOwner(ctx context.Context, pod *corev1.Pod) (*appsv1.StatefulSet, error) {
	ownerReferences := pod.OwnerReferences

	if len(ownerReferences) == 0 {
		return nil, errors.New("there are no owners")
	}

	for _, owner := range ownerReferences {
		if owner.Kind == "StatefulSet" {
			return h.StatefulSetGetter.Get(ctx, pod.Namespace, owner.Name)
		}
	}

	return nil, errors.New("there are no statefulset owners")
}

func isReady(conditions []corev1.PodCondition) bool {
	for _, c := range conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

func decodeRoutes(s string) ([]cf.Route, error) {
	routes := []cf.Route{}
	err := json.Unmarshal([]byte(s), &routes)

	return routes, errors.Wrap(err, "failed to unmarshal routes")
}

func markedForDeletion(pod corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}
