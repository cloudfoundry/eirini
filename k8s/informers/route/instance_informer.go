package route

import (
	"code.cloudfoundry.org/eirini/route"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const NoResync = 0

//go:generate counterfeiter . UpdateEventHandler
type UpdateEventHandler interface {
	Handle(oldObj, updatedObj interface{})
}

type InstanceChangeInformer struct {
	Cancel        <-chan struct{}
	Client        kubernetes.Interface
	Namespace     string
	UpdateHandler UpdateEventHandler
}

func NewInstanceChangeInformer(client kubernetes.Interface, namespace string, updateHandler UpdateEventHandler) route.Informer {
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
		UpdateFunc: c.UpdateHandler.Handle,
	})

	podInformer.Run(c.Cancel)
}
