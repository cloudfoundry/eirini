package task

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/lager"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Reporter interface {
	Report(*v1.Pod, *v1.Pod)
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

	informer := factory.Core().V1().Pods().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updateFunc,
	})

	informer.Run(c.stopperChan)
}

func (c *Informer) updateFunc(oldObj interface{}, newObj interface{}) {
	oldPod := oldObj.(*v1.Pod)
	pod := newObj.(*v1.Pod)
	c.reporter.Report(oldPod, pod)
}

func tweakListOpts(opts *metav1.ListOptions) {
	opts.LabelSelector = fmt.Sprintf(
		"%s=%s",
		k8s.LabelSourceType, "TASK",
	)
}
