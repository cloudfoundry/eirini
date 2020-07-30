package event

import (
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/lager"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const CrashLoopBackOff = "CrashLoopBackOff"
const CreateContainerConfigError = "CreateContainerConfigError"

//counterfeiter:generate . CrashEventGenerator
//counterfeiter:generate . CrashEmitter

type CrashEventGenerator interface {
	Generate(*v1.Pod, lager.Logger) (events.CrashEvent, bool)
}

type CrashEmitter interface {
	Emit(events.CrashEvent) error
}

type CrashInformer struct {
	clientset      kubernetes.Interface
	syncPeriod     time.Duration
	namespace      string
	stopperChan    chan struct{}
	logger         lager.Logger
	eventGenerator CrashEventGenerator
	crashEmitter   CrashEmitter
}

func NewCrashInformer(
	client kubernetes.Interface,
	syncPeriod time.Duration,
	namespace string,
	stopperChan chan struct{},
	logger lager.Logger,
	eventGenerator CrashEventGenerator,
	crashEmitter CrashEmitter,
) *CrashInformer {
	return &CrashInformer{
		clientset:      client,
		syncPeriod:     syncPeriod,
		namespace:      namespace,
		stopperChan:    stopperChan,
		logger:         logger,
		eventGenerator: eventGenerator,
		crashEmitter:   crashEmitter,
	}
}

func (c *CrashInformer) Start() {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.clientset,
		c.syncPeriod,
		informers.WithNamespace(c.namespace),
	)

	informer := factory.Core().V1().Pods().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updateFunc,
	})

	informer.Run(c.stopperChan)
}

func (c *CrashInformer) updateFunc(_ interface{}, newObj interface{}) {
	pod := newObj.(*v1.Pod)
	event, send := c.eventGenerator.Generate(pod, c.logger)
	if send {
		if err := c.crashEmitter.Emit(event); err != nil {
			c.logger.Error("failed-to-emit-crash-event", err)
		}
	}
}
