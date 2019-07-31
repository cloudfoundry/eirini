package route

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const NoResync = 0

type InstanceChangeInformer struct {
	Cancel    <-chan struct{}
	Client    kubernetes.Interface
	Namespace string
	Logger    lager.Logger
}

func NewInstanceChangeInformer(client kubernetes.Interface, namespace string, logger lager.Logger) route.Informer {
	return &InstanceChangeInformer{
		Client:    client,
		Namespace: namespace,
		Cancel:    make(<-chan struct{}),
		Logger:    logger,
	}
}

func (c *InstanceChangeInformer) Start(work chan<- *eiriniroute.Message) {
	factory := informers.NewSharedInformerFactoryWithOptions(c.Client,
		NoResync,
		informers.WithNamespace(c.Namespace))

	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, updatedObj interface{}) {
			c.onPodUpdate(oldObj, updatedObj, work)
		},
	})

	podInformer.Run(c.Cancel)
}

func (c *InstanceChangeInformer) onPodUpdate(oldObj, updatedObj interface{}, work chan<- *eiriniroute.Message) {
	updatedPod := updatedObj.(*v1.Pod)
	oldPod := oldObj.(*v1.Pod)
	loggerSession := c.Logger.Session("pod-update", lager.Data{"pod-name": updatedPod.Name, "guid": updatedPod.Annotations[cf.ProcessGUID]})

	userDefinedRoutes, err := c.getUserDefinedRoutes(updatedPod)
	if err != nil {
		loggerSession.Debug("failed-to-get-user-defined-routes", lager.Data{"error": err.Error()})
		return
	}

	if markedForDeletion(updatedPod) || !isReady(updatedPod.Status.Conditions) && isReady(oldPod.Status.Conditions) {
		loggerSession.Debug("pod-not-ready", lager.Data{"statuses": updatedPod.Status.Conditions, "deletion-timestamp": updatedPod.DeletionTimestamp})
		c.unregisterPodRoutes(oldPod, userDefinedRoutes, work)
		return
	}

	for _, r := range userDefinedRoutes {
		routes, err := NewRouteMessage(
			updatedPod,
			uint32(r.Port),
			eiriniroute.Routes{RegisteredRoutes: []string{r.Hostname}},
		)
		if err != nil {
			loggerSession.Debug("failed-to-construct-a-route-message", lager.Data{"error": err.Error()})
			continue
		}
		work <- routes
	}
}

func (c *InstanceChangeInformer) unregisterPodRoutes(pod *v1.Pod, userDefinedRoutes []cf.Route, work chan<- *eiriniroute.Message) {
	loggerSession := c.Logger.Session("pod-delete", lager.Data{"pod-name": pod.Name, "guid": pod.Annotations[cf.ProcessGUID]})

	for _, r := range userDefinedRoutes {
		routes, err := NewRouteMessage(
			pod,
			uint32(r.Port),
			eiriniroute.Routes{UnregisteredRoutes: []string{r.Hostname}},
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
