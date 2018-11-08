package route

import (
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	set "github.com/deckarep/golang-set"
	apps_v1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type URIChangeInformer struct {
	Cancel     <-chan struct{}
	Client     kubernetes.Interface
	SyncPeriod time.Duration
	Namespace  string
	Logger     lager.Logger
}

func NewURIChangeInformer(client kubernetes.Interface, syncPeriod time.Duration, namespace string) route.Informer {
	return &URIChangeInformer{
		Client:     client,
		SyncPeriod: syncPeriod,
		Namespace:  namespace,
		Cancel:     make(<-chan struct{}),
		Logger:     lager.NewLogger("uri-change-informer"),
	}
}

func (c *URIChangeInformer) Start(work chan<- *route.Message) {
	factory := informers.NewSharedInformerFactoryWithOptions(c.Client,
		c.SyncPeriod,
		informers.WithNamespace(c.Namespace))

	informer := factory.Apps().V1().StatefulSets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, updatedObj interface{}) {
			c.onUpdate(oldObj, updatedObj, work)
		},
	})

	informer.Run(c.Cancel)
}

func (c *URIChangeInformer) onUpdate(oldObj, updatedObj interface{}, work chan<- *route.Message) {
	oldStatefulSet := oldObj.(*apps_v1.StatefulSet)
	updatedStatefulSet := updatedObj.(*apps_v1.StatefulSet)

	updatedSet, err := decodeRoutesAsSet(updatedStatefulSet)
	if err != nil {
		c.logError("failed-to-decode-updated-user-defined-routes", err, updatedStatefulSet)
	}

	oldSet, err := decodeRoutesAsSet(oldStatefulSet)
	if err != nil {
		c.logError("failed-to-decode-old-user-defined-routes", err, oldStatefulSet)
	}

	removedRoutes := oldSet.Difference(updatedSet)

	pods, err := c.getChildrenPods(updatedStatefulSet)
	if err != nil {
		c.logError("failed-to-get-child-pods", err, updatedStatefulSet)
		return
	}
	for _, pod := range pods {
		podRoute, err := route.NewMessage(
			pod.Name,
			pod.Name,
			pod.Status.PodIP,
			getContainerPort(&pod),
		)
		if err != nil {
			c.logPodError("failed-to-construct-a-route-message", err, updatedStatefulSet, &pod)
			return
		}
		podRoute.Routes = toStringSlice(updatedSet)
		podRoute.UnregisteredRoutes = toStringSlice(removedRoutes)
		work <- podRoute
	}
}

func (c *URIChangeInformer) logError(message string, err error, statefulset *apps_v1.StatefulSet) {
	if c.Logger != nil {
		c.Logger.Error(message, err, lager.Data{"statefulset-name": statefulset.Name})
	}
}

func (c *URIChangeInformer) logPodError(message string, err error, statefulset *apps_v1.StatefulSet, pod *v1.Pod) {
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

func toStringSlice(routes set.Set) []string {
	slice := []string{}
	for _, r := range routes.ToSlice() {
		slice = append(slice, r.(string))
	}
	return slice
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
