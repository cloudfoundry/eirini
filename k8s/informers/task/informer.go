package task

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/lager"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Reporter interface {
	Report(*batchv1.Job)
}

type Informer struct {
	clientset   kubernetes.Interface
	syncPeriod  time.Duration
	namespace   string
	reporter    Reporter
	stopperChan chan struct{}
	logger      lager.Logger
}

func NewInformer(
	client kubernetes.Interface,
	syncPeriod time.Duration,
	namespace string,
	reporter Reporter,
	stopperChan chan struct{},
	logger lager.Logger,
) *Informer {
	return &Informer{
		clientset:   client,
		syncPeriod:  syncPeriod,
		namespace:   namespace,
		reporter:    reporter,
		stopperChan: stopperChan,
		logger:      logger,
	}
}

func (c *Informer) Start() {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.clientset,
		c.syncPeriod,
		informers.WithNamespace(c.namespace),
		informers.WithTweakListOptions(tweakListOpts),
	)

	informer := factory.Batch().V1().Jobs().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updateFunc,
	})

	informer.Run(c.stopperChan)
}

func (c *Informer) updateFunc(_ interface{}, newObj interface{}) {
	job := newObj.(*batchv1.Job)
	c.reporter.Report(job)
}

func tweakListOpts(opts *metav1.ListOptions) {
	opts.LabelSelector = fmt.Sprintf(
		"%s=%s",
		k8s.LabelSourceType, "TASK",
	)
}
