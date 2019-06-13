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

type CrashInformer struct {
	clientset       kubernetes.Interface
	syncPeriod      time.Duration
	namespace       string
	reportChan      chan events.CrashReport
	stopperChan     chan struct{}
	logger          lager.Logger
	reportGenerator CrashReportGenerator
}

type CrashReportGenerator interface {
	Generate(*v1.Pod, kubernetes.Interface, lager.Logger) (events.CrashReport, bool)
}

func NewCrashInformer(
	client kubernetes.Interface,
	syncPeriod time.Duration,
	namespace string,
	reportChan chan events.CrashReport,
	stopperChan chan struct{},
	logger lager.Logger,
	reportGenerator CrashReportGenerator,
) *CrashInformer {
	return &CrashInformer{
		clientset:       client,
		syncPeriod:      syncPeriod,
		namespace:       namespace,
		reportChan:      reportChan,
		stopperChan:     stopperChan,
		logger:          logger,
		reportGenerator: reportGenerator,
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
	report, send := c.reportGenerator.Generate(pod, c.clientset, c.logger)
	if send {
		c.reportChan <- report
	}
}
