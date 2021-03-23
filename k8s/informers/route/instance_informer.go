package route

import (
	"context"

	"code.cloudfoundry.org/eirini/route"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const NoResync = 0

//counterfeiter:generate . PodUpdateEventHandler

type PodUpdateEventHandler interface {
	Handle(ctx context.Context, oldObj, updatedObj *v1.Pod)
}

type InstanceChangeInformer struct {
	Cancel        <-chan struct{}
	Client        kubernetes.Interface
	Namespace     string
	UpdateHandler PodUpdateEventHandler
}

func NewInstanceChangeInformer(client kubernetes.Interface, namespace string, updateHandler PodUpdateEventHandler) route.Informer {
	return &InstanceChangeInformer{
		Client:        client,
		Namespace:     namespace,
		UpdateHandler: updateHandler,
		Cancel:        make(<-chan struct{}),
	}
}

func (c *InstanceChangeInformer) Start() {
	factory := informers.NewSharedInformerFactoryWithOptions(c.Client,
		NoResync,
		informers.WithNamespace(c.Namespace))

	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, updatedObj interface{}) {
			oldPod := oldObj.(*v1.Pod)         //nolint:forcetypeassert
			updatedPod := updatedObj.(*v1.Pod) //nolint:forcetypeassert
			c.UpdateHandler.Handle(context.Background(), oldPod, updatedPod)
		},
	})

	podInformer.Run(c.Cancel)
}
