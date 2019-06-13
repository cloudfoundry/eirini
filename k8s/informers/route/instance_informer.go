package route

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type InstanceChangeInformer struct {
	Cancel     <-chan struct{}
	Client     kubernetes.Interface
	SyncPeriod time.Duration
	Namespace  string
	Logger     lager.Logger
}

func NewInstanceChangeInformer(client kubernetes.Interface, syncPeriod time.Duration, namespace string, logger lager.Logger) route.Informer {
	return &InstanceChangeInformer{
		Client:     client,
		SyncPeriod: syncPeriod,
		Namespace:  namespace,
		Cancel:     make(<-chan struct{}),
		Logger:     logger,
	}
}

func (c *InstanceChangeInformer) Start(work chan<- *route.Message) {
	factory := informers.NewSharedInformerFactoryWithOptions(c.Client,
		c.SyncPeriod,
		informers.WithNamespace(c.Namespace))

	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_ interface{}, updatedObj interface{}) {
			c.onPodUpdate(updatedObj, work)
		},
		DeleteFunc: func(obj interface{}) {
			c.onPodDelete(obj, work)
		},
	})

	podInformer.Run(c.Cancel)
}

func (c *InstanceChangeInformer) onPodDelete(deletedObj interface{}, work chan<- *route.Message) {
	deletedPod := deletedObj.(*v1.Pod)
	loggerSession := c.Logger.Session("pod-delete", lager.Data{"pod-name": deletedPod.Name, "guid": deletedPod.Annotations[cf.ProcessGUID]})
	userDefinedRoutes, err := c.getUserDefinedRoutes(deletedPod)
	if err != nil {
		loggerSession.Debug("failed-to-get-user-defined-routes", lager.Data{"error": err.Error()})
		return
	}

	for _, r := range userDefinedRoutes {
		routes, err := NewRouteMessage(
			deletedPod,
			uint32(r.Port),
			route.Routes{UnregisteredRoutes: []string{r.Hostname}},
		)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})
			continue
		}
		work <- routes
	}
}

func (c *InstanceChangeInformer) onPodUpdate(updatedObj interface{}, work chan<- *route.Message) {
	updatedPod := updatedObj.(*v1.Pod)
	loggerSession := c.Logger.Session("pod-update", lager.Data{"pod-name": updatedPod.Name, "guid": updatedPod.Annotations[cf.ProcessGUID]})
	if !isReady(updatedPod.Status.Conditions) {
		loggerSession.Debug("pod-status-not-ready", lager.Data{"statuses": updatedPod.Status.Conditions})
		return
	}

	userDefinedRoutes, err := c.getUserDefinedRoutes(updatedPod)
	if err != nil {
		loggerSession.Debug("failed-to-get-user-defined-routes", lager.Data{"error": err.Error()})
		return
	}

	for _, r := range userDefinedRoutes {
		routes, err := NewRouteMessage(
			updatedPod,
			uint32(r.Port),
			route.Routes{RegisteredRoutes: []string{r.Hostname}},
		)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})
			continue
		}
		work <- routes
	}
}

func (c *InstanceChangeInformer) getUserDefinedRoutes(pod *v1.Pod) ([]cf.Route, error) {
	owner, err := c.getOwner(pod)
	if err != nil {
		return []cf.Route{}, errors.Wrap(err, "failed to get owner")
	}

	return decodeRoutes(owner.Annotations[eirini.RegisteredRoutes])
}

func (c *InstanceChangeInformer) getOwner(pod *v1.Pod) (*apps.StatefulSet, error) {
	ownerReferences := pod.OwnerReferences

	if len(ownerReferences) == 0 {
		return nil, errors.New("there are no owners")
	}
	for _, owner := range ownerReferences {
		if owner.Kind == "StatefulSet" {
			return c.Client.AppsV1().StatefulSets(c.Namespace).Get(owner.Name, meta.GetOptions{})
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
