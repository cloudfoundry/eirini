package event

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/models/cf"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/client-go/kubernetes/typed/apps/v1"
)

//go:generate counterfeiter -o eventfakes/fake_statefulset_interface.go ../../../../vendor/k8s.io/client-go/kubernetes/typed/apps/v1/statefulset.go StatefulSetInterface

type PodUpdateHandler struct {
	Client       types.StatefulSetInterface
	Logger       lager.Logger
	RouteEmitter eiriniroute.Emitter
}

func (h PodUpdateHandler) Handle(oldObj, updatedObj interface{}) {
	updatedPod := updatedObj.(*v1.Pod)
	oldPod := oldObj.(*v1.Pod)
	loggerSession := h.Logger.Session("pod-update", lager.Data{"pod-name": updatedPod.Name, "guid": updatedPod.Annotations[k8s.AnnotationProcessGUID]})

	userDefinedRoutes, err := h.getUserDefinedRoutes(updatedPod)
	if err != nil {
		loggerSession.Debug("failed-to-get-user-defined-routes", lager.Data{"error": err.Error()})
		return
	}

	if markedForDeletion(*updatedPod) || (!isReady(updatedPod.Status.Conditions) && isReady(oldPod.Status.Conditions)) {
		loggerSession.Debug("pod-not-ready", lager.Data{"statuses": updatedPod.Status.Conditions, "deletion-timestamp": updatedPod.DeletionTimestamp})
		h.unregisterPodRoutes(oldPod, userDefinedRoutes)
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
		h.RouteEmitter.Emit(*routes)
	}
}

func (h PodUpdateHandler) unregisterPodRoutes(pod *v1.Pod, userDefinedRoutes []cf.Route) {
	loggerSession := h.Logger.Session("pod-delete", lager.Data{"pod-name": pod.Name, "guid": pod.Annotations[k8s.AnnotationProcessGUID]})

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
		h.RouteEmitter.Emit(*routes)
	}
}

func (h PodUpdateHandler) getUserDefinedRoutes(pod *v1.Pod) ([]cf.Route, error) {
	owner, err := h.getOwner(pod)
	if err != nil {
		return []cf.Route{}, errors.Wrap(err, "failed to get owner")
	}

	return decodeRoutes(owner.Annotations[k8s.AnnotationRegisteredRoutes])
}

func (h PodUpdateHandler) getOwner(pod *v1.Pod) (*apps.StatefulSet, error) {
	ownerReferences := pod.OwnerReferences

	if len(ownerReferences) == 0 {
		return nil, errors.New("there are no owners")
	}
	for _, owner := range ownerReferences {
		if owner.Kind == "StatefulSet" {
			return h.Client.Get(owner.Name, meta.GetOptions{})
		}
	}

	return nil, errors.New("there are no statefulset owners")
}

func isReady(conditions []v1.PodCondition) bool {
	for _, c := range conditions {
		if c.Type == v1.PodReady {
			return c.Status == v1.ConditionTrue
		}
	}
	return false
}

func decodeRoutes(s string) ([]cf.Route, error) {
	routes := []cf.Route{}
	err := json.Unmarshal([]byte(s), &routes)

	return routes, err
}

func markedForDeletion(pod corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}
