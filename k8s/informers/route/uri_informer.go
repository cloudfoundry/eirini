package route

import (
	"errors"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	set "github.com/deckarep/golang-set"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type portGroup map[int32]route.Routes

type URIChangeInformer struct {
	Cancel     <-chan struct{}
	Client     kubernetes.Interface
	SyncPeriod time.Duration
	Namespace  string
	Logger     lager.Logger
}

func NewURIChangeInformer(client kubernetes.Interface, syncPeriod time.Duration, namespace string, logger lager.Logger) route.Informer {
	return &URIChangeInformer{
		Client:     client,
		SyncPeriod: syncPeriod,
		Namespace:  namespace,
		Cancel:     make(<-chan struct{}),
		Logger:     logger,
	}
}

func NewRouteMessage(pod *v1.Pod, port uint32, routes route.Routes) (*route.Message, error) {
	if len(pod.Status.PodIP) == 0 {
		return nil, errors.New("missing ip address")
	}

	message := &route.Message{
		Routes: route.Routes{
			UnregisteredRoutes: routes.UnregisteredRoutes,
		},
		Name:       pod.Name,
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

func (c *URIChangeInformer) Start(work chan<- *route.Message) {
	factory := informers.NewSharedInformerFactoryWithOptions(c.Client,
		c.SyncPeriod,
		informers.WithNamespace(c.Namespace))

	informer := factory.Apps().V1().StatefulSets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, updatedObj interface{}) {
			c.onUpdate(oldObj, updatedObj, work)
		},
		DeleteFunc: func(obj interface{}) {
			c.onDelete(obj, work)
		},
	})

	informer.Run(c.Cancel)
}

func (c *URIChangeInformer) onUpdate(oldObj, updatedObj interface{}, work chan<- *route.Message) {
	oldStatefulSet := oldObj.(*apps_v1.StatefulSet)
	updatedStatefulSet := updatedObj.(*apps_v1.StatefulSet)

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

	c.sendRoutesForAllPods(
		loggerSession,
		work,
		updatedStatefulSet,
		grouped,
	)
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

func (c *URIChangeInformer) onDelete(obj interface{}, work chan<- *route.Message) {
	deletedStatefulSet := obj.(*apps_v1.StatefulSet)
	loggerSession := c.Logger.Session("statefulset-delete", lager.Data{"guid": deletedStatefulSet.Spec.Template.Annotations[cf.ProcessGUID]})

	routeSet, err := decodeRoutesAsSet(deletedStatefulSet)
	if err != nil {
		loggerSession.Error("failed-to-decode-deleted-user-defined-routes", err)
	}

	routeGroups := groupRoutesByPort(routeSet, set.NewSet())
	c.sendRoutesForAllPods(
		loggerSession,
		work,
		deletedStatefulSet,
		routeGroups,
	)
}

func (c *URIChangeInformer) sendRoutesForAllPods(loggerSession lager.Logger, work chan<- *route.Message, statefulset *apps_v1.StatefulSet, grouped portGroup) {
	pods, err := c.getChildrenPods(statefulset)
	if err != nil {
		loggerSession.Error("failed-to-get-child-pods", err)
		return
	}

	for _, pod := range pods {
		pod := pod
		for port, routes := range grouped {
			podRoute, err := NewRouteMessage(&pod, uint32(port), routes)
			if err != nil {
				loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})
				continue
			}
			work <- podRoute
		}
	}
}

func (c *URIChangeInformer) logError(message string, err error, statefulset *apps_v1.StatefulSet) {
	if c.Logger != nil {
		c.Logger.Error(message, err, lager.Data{"statefulset-name": statefulset.Name})
	}
}

func (c *URIChangeInformer) logPodError(message string, err error, statefulset *apps_v1.StatefulSet, pod v1.Pod) {
	if c.Logger != nil {
		c.Logger.Error(message, err, lager.Data{"statefulset-name": statefulset.Name, "pod-name": pod.Name})
	}
}

func (c *URIChangeInformer) getChildrenPods(st *apps_v1.StatefulSet) ([]v1.Pod, error) {
	set := labels.Set(st.Spec.Selector.MatchLabels)
	opts := meta.ListOptions{LabelSelector: set.AsSelector().String()}
	podlist, err := c.Client.CoreV1().Pods(c.Namespace).List(opts)
	if err != nil {
		return []v1.Pod{}, err
	}
	return podlist.Items, nil
}

func decodeRoutesAsSet(statefulset *apps_v1.StatefulSet) (set.Set, error) {
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
