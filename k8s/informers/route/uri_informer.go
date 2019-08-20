package route

import (
	"errors"
	"reflect"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	set "github.com/deckarep/golang-set"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type portGroup map[int32]eiriniroute.Routes

type URIChangeInformer struct {
	Cancel    <-chan struct{}
	Client    kubernetes.Interface
	Namespace string
	Logger    lager.Logger
}

func NewURIChangeInformer(client kubernetes.Interface, namespace string, logger lager.Logger) route.Informer {
	return &URIChangeInformer{
		Client:    client,
		Namespace: namespace,
		Cancel:    make(<-chan struct{}),
		Logger:    logger,
	}
}

func NewRouteMessage(pod *corev1.Pod, port uint32, routes eiriniroute.Routes) (*eiriniroute.Message, error) {
	if len(pod.Status.PodIP) == 0 {
		return nil, errors.New("missing ip address")
	}

	message := &eiriniroute.Message{
		Routes: eiriniroute.Routes{
			UnregisteredRoutes: routes.UnregisteredRoutes,
		},
		Name:       pod.Labels["guid"],
		InstanceID: pod.Name,
		Address:    pod.Status.PodIP,
		Port:       port,
		TLSPort:    0,
	}
	if isReady(pod.Status.Conditions) {
		message.RegisteredRoutes = routes.RegisteredRoutes
	}

	if len(message.RegisteredRoutes) == 0 && len(message.UnregisteredRoutes) == 0 {
		return nil, errors.New("no-routes-provided")
	}

	return message, nil
}

func (c *URIChangeInformer) Start(work chan<- *eiriniroute.Message) {
	factory := informers.NewSharedInformerFactoryWithOptions(c.Client,
		NoResync,
		informers.WithNamespace(c.Namespace))

	informer := factory.Apps().V1().StatefulSets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, updatedObj interface{}) {
			c.onAnnotationUpdate(oldObj, updatedObj, work)
		},
		DeleteFunc: func(obj interface{}) {
			c.onDelete(obj, work)
		},
	})

	informer.Run(c.Cancel)
}

func (c *URIChangeInformer) onAnnotationUpdate(oldObj, updatedObj interface{}, work chan<- *eiriniroute.Message) {
	oldStatefulSet := oldObj.(*appsv1.StatefulSet)
	updatedStatefulSet := updatedObj.(*appsv1.StatefulSet)

	if reflect.DeepEqual(oldStatefulSet.Annotations, updatedStatefulSet.Annotations) {
		return
	}

	c.onUpdate(oldStatefulSet, updatedStatefulSet, work)
}

func (c *URIChangeInformer) onUpdate(oldStatefulSet, updatedStatefulSet *appsv1.StatefulSet, work chan<- *eiriniroute.Message) {
	loggerSession := c.Logger.Session("statefulset-update", lager.Data{"guid": updatedStatefulSet.Spec.Template.Annotations[cf.ProcessGUID]})

	updatedSet, err := decodeRoutesAsSet(updatedStatefulSet)
	if err != nil {
		loggerSession.Error("failed-to-decode-updated-user-defined-routes", err)
	}

	oldSet, err := decodeRoutesAsSet(oldStatefulSet)
	if err != nil {
		loggerSession.Error("failed-to-decode-old-user-defined-routes", err)
	}

	removedRoutes := oldSet.Difference(updatedSet)
	grouped := groupRoutesByPort(removedRoutes, updatedSet)

	routes := c.createRoutesOnUpdate(
		loggerSession,
		updatedStatefulSet,
		grouped,
	)
	for _, route := range routes {
		work <- route
	}
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

func (c *URIChangeInformer) onDelete(obj interface{}, work chan<- *eiriniroute.Message) {
	deletedStatefulSet := obj.(*appsv1.StatefulSet)
	loggerSession := c.Logger.Session("statefulset-delete", lager.Data{"guid": deletedStatefulSet.Spec.Template.Annotations[cf.ProcessGUID]})

	routeSet, err := decodeRoutesAsSet(deletedStatefulSet)
	if err != nil {
		loggerSession.Error("failed-to-decode-deleted-user-defined-routes", err)
	}

	routeGroups := groupRoutesByPort(routeSet, set.NewSet())
	routes := c.createRoutesOnDelete(
		loggerSession,
		deletedStatefulSet,
		routeGroups,
	)
	for _, route := range routes {
		work <- route
	}
}

func (c *URIChangeInformer) createRoutesOnUpdate(loggerSession lager.Logger, statefulset *appsv1.StatefulSet, grouped portGroup) []*route.Message {
	pods, err := c.getChildrenPods(statefulset)
	if err != nil {
		loggerSession.Error("failed-to-get-child-pods", err)
		return []*route.Message{}
	}

	resultRoutes := []*route.Message{}
	for _, pod := range pods {
		if markedForDeletion(pod) {
			loggerSession.Debug("skipping pod marked for deletion")
			continue
		}
		resultRoutes = append(resultRoutes, createRouteMessages(loggerSession, pod, grouped)...)
	}
	return resultRoutes
}

func (c *URIChangeInformer) createRoutesOnDelete(loggerSession lager.Logger, statefulset *appsv1.StatefulSet, grouped portGroup) []*route.Message {
	pods, err := c.getChildrenPods(statefulset)
	if err != nil {
		loggerSession.Error("failed-to-get-child-pods", err)
		return []*route.Message{}
	}

	resultRoutes := []*route.Message{}
	for _, pod := range pods {
		resultRoutes = append(resultRoutes, createRouteMessages(loggerSession, pod, grouped)...)
	}
	return resultRoutes
}

func createRouteMessages(loggerSession lager.Logger, pod corev1.Pod, grouped portGroup) []*route.Message {
	resultRoutes := []*route.Message{}
	for port, routes := range grouped {
		podRoute, err := NewRouteMessage(&pod, uint32(port), routes)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})
			continue
		}
		resultRoutes = append(resultRoutes, podRoute)
	}
	return resultRoutes
}

func markedForDeletion(pod corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func (c *URIChangeInformer) getChildrenPods(st *appsv1.StatefulSet) ([]corev1.Pod, error) {
	set := labels.Set(st.Spec.Selector.MatchLabels)
	opts := metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	podlist, err := c.Client.CoreV1().Pods(c.Namespace).List(opts)
	if err != nil {
		return []corev1.Pod{}, err
	}
	return podlist.Items, nil
}

func decodeRoutesAsSet(statefulset *appsv1.StatefulSet) (set.Set, error) {
	routes := set.NewSet()
	updatedUserDefinedRoutes, err := decodeRoutes(statefulset.Annotations[eirini.RegisteredRoutes])
	if err != nil {
		return set.NewSet(), err
	}

	for _, r := range updatedUserDefinedRoutes {
		routes.Add(r)
	}
	return routes, nil
}
