package event

import (
	"reflect"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/route"
	"code.cloudfoundry.org/eirini/models/cf"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	set "github.com/deckarep/golang-set"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

//counterfeiter:generate -o eventfakes/fake_pod_interface.go ../../../../vendor/k8s.io/client-go/kubernetes/typed/core/v1/pod.go PodInterface

type portGroup map[int32]eiriniroute.Routes

type URIAnnotationUpdateHandler struct {
	Pods         typedv1.PodInterface
	Logger       lager.Logger
	RouteEmitter eiriniroute.Emitter
}

func (h URIAnnotationUpdateHandler) Handle(oldStatefulSet, updatedStatefulSet *appsv1.StatefulSet) {
	if reflect.DeepEqual(oldStatefulSet.Annotations, updatedStatefulSet.Annotations) {
		return
	}

	h.onUpdate(oldStatefulSet, updatedStatefulSet)
}

func (h URIAnnotationUpdateHandler) onUpdate(oldStatefulSet, updatedStatefulSet *appsv1.StatefulSet) {
	loggerSession := h.Logger.Session("statefulset-update", lager.Data{"guid": updatedStatefulSet.Annotations[k8s.AnnotationProcessGUID]})

	updatedSet, err := decodeRoutesAsSet(updatedStatefulSet)
	if err != nil {
		loggerSession.Error("failed-to-decode-updated-user-defined-routes", err)
		return
	}

	oldSet, err := decodeRoutesAsSet(oldStatefulSet)
	if err != nil {
		loggerSession.Error("failed-to-decode-old-user-defined-routes", err)
	}

	removedRoutes := oldSet.Difference(updatedSet)
	grouped := groupRoutesByPort(removedRoutes, updatedSet)

	routes := h.createRoutesOnUpdate(
		loggerSession,
		updatedStatefulSet,
		grouped,
	)
	for _, route := range routes {
		h.RouteEmitter.Emit(*route)
	}
}

func (h URIAnnotationUpdateHandler) createRoutesOnUpdate(loggerSession lager.Logger, statefulset *appsv1.StatefulSet, grouped portGroup) []*eiriniroute.Message {
	pods, err := getChildrenPods(h.Pods, statefulset)
	if err != nil {
		loggerSession.Error("failed-to-get-child-pods", err)
		return []*eiriniroute.Message{}
	}

	resultRoutes := []*eiriniroute.Message{}
	for _, pod := range pods {
		if markedForDeletion(pod) {
			loggerSession.Debug("skipping pod marked for deletion")
			continue
		}
		resultRoutes = append(resultRoutes, createRouteMessages(loggerSession, pod, grouped)...)
	}
	return resultRoutes
}

func createRouteMessages(loggerSession lager.Logger, pod corev1.Pod, grouped portGroup) []*eiriniroute.Message {
	resultRoutes := []*eiriniroute.Message{}
	for port, routes := range grouped {
		podRoute, err := route.NewRouteMessage(&pod, uint32(port), routes)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})
			continue
		}
		resultRoutes = append(resultRoutes, podRoute)
	}
	return resultRoutes
}

func groupRoutesByPort(remove, add set.Set) portGroup {
	group := make(portGroup)
	for _, toAdd := range add.ToSlice() {
		current := toAdd.(cf.Route)
		routes := group[current.Port]
		routes.RegisteredRoutes = append(routes.RegisteredRoutes, current.Hostname)
		group[current.Port] = routes
	}
	for _, toRemove := range remove.ToSlice() {
		current := toRemove.(cf.Route)
		routes := group[current.Port]
		routes.UnregisteredRoutes = append(routes.UnregisteredRoutes, current.Hostname)
		group[current.Port] = routes
	}

	return group
}

func decodeRoutesAsSet(statefulset *appsv1.StatefulSet) (set.Set, error) {
	routes := set.NewSet()
	updatedUserDefinedRoutes, err := decodeRoutes(statefulset.Annotations[k8s.AnnotationRegisteredRoutes])
	if err != nil {
		return set.NewSet(), err
	}

	for _, r := range updatedUserDefinedRoutes {
		routes.Add(r)
	}
	return routes, nil
}
