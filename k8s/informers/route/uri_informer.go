package route

import (
	"errors"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/route"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

//go:generate counterfeiter . DeleteEventHandler
type DeleteEventHandler interface {
	Handle(obj interface{})
}

type portGroup map[int32]eiriniroute.Routes

type URIChangeInformer struct {
	Cancel        <-chan struct{}
	Client        kubernetes.Interface
	UpdateHandler UpdateEventHandler
	DeleteHandler DeleteEventHandler
	Namespace     string
}

func NewURIChangeInformer(client kubernetes.Interface, namespace string, updateEventHandler UpdateEventHandler, deleteEventHandler DeleteEventHandler) route.Informer {
	return &URIChangeInformer{
		Client:        client,
		Namespace:     namespace,
		Cancel:        make(<-chan struct{}),
		UpdateHandler: updateEventHandler,
		DeleteHandler: deleteEventHandler,
	}
}

func (i *URIChangeInformer) Start() {
	factory := informers.NewSharedInformerFactoryWithOptions(i.Client,
		NoResync,
		informers.WithNamespace(i.Namespace))

	informer := factory.Apps().V1().StatefulSets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: i.UpdateHandler.Handle,
		DeleteFunc: i.DeleteHandler.Handle,
	})

	informer.Run(i.Cancel)
}

func NewRouteMessage(pod *corev1.Pod, port uint32, routes eiriniroute.Routes) (*eiriniroute.Message, error) {
	if len(pod.Status.PodIP) == 0 {
		return nil, errors.New("missing ip address")
	}

	message := &eiriniroute.Message{
		Routes: eiriniroute.Routes{
			UnregisteredRoutes: routes.UnregisteredRoutes,
		},
		Name:       pod.Labels[k8s.LabelGUID],
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

func isReady(conditions []v1.PodCondition) bool {
	for _, c := range conditions {
		if c.Type == v1.PodReady {
			return c.Status == v1.ConditionTrue
		}
	}
	return false
}
