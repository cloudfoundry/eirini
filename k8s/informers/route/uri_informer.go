package route

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/k8s/stset"
	eiriniroute "code.cloudfoundry.org/eirini/route"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

//counterfeiter:generate . StatefulSetUpdateEventHandler
//counterfeiter:generate . StatefulSetDeleteEventHandler

type StatefulSetUpdateEventHandler interface {
	Handle(ctx context.Context, oldObj, updatedObj *appsv1.StatefulSet)
}

type StatefulSetDeleteEventHandler interface {
	Handle(ctx context.Context, obj *appsv1.StatefulSet)
}

type URIChangeInformer struct {
	Cancel        <-chan struct{}
	Client        kubernetes.Interface
	UpdateHandler StatefulSetUpdateEventHandler
	DeleteHandler StatefulSetDeleteEventHandler
	Namespace     string
}

func NewURIChangeInformer(client kubernetes.Interface, namespace string, updateEventHandler StatefulSetUpdateEventHandler, deleteEventHandler StatefulSetDeleteEventHandler) eiriniroute.Informer {
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

	ctx := context.Background()

	informer := factory.Apps().V1().StatefulSets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, updatedObj interface{}) {
			oldStatefulSet := oldObj.(*appsv1.StatefulSet)         //nolint:forcetypeassert
			updatedStatefulSet := updatedObj.(*appsv1.StatefulSet) //nolint:forcetypeassert
			i.UpdateHandler.Handle(ctx, oldStatefulSet, updatedStatefulSet)
		},
		DeleteFunc: func(obj interface{}) {
			statefulSet := obj.(*appsv1.StatefulSet) //nolint:forcetypeassert
			i.DeleteHandler.Handle(ctx, statefulSet)
		},
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
		Name:       pod.Labels[stset.LabelGUID],
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

func isReady(conditions []corev1.PodCondition) bool {
	for _, c := range conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}
