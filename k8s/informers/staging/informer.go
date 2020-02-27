package staging

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

type StagingReporter interface {
	Report(*v1.Pod)
}

type StagingInformer struct {
	clientset   kubernetes.Interface
	syncPeriod  time.Duration
	namespace   string
	reporter    StagingReporter
	stopperChan chan struct{}
	logger      lager.Logger
}

func NewInformer(
	client kubernetes.Interface,
	syncPeriod time.Duration,
	namespace string,
	reporter StagingReporter,
	stopperChan chan struct{},
	logger lager.Logger,
) *StagingInformer {
	return &StagingInformer{
		clientset:   client,
		syncPeriod:  syncPeriod,
		namespace:   namespace,
		reporter:    reporter,
		stopperChan: stopperChan,
		logger:      logger,
	}
}

func (c *StagingInformer) Start() {
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

func (c *StagingInformer) updateFunc(_ interface{}, newObj interface{}) {
	pod := newObj.(*v1.Pod)
	fmt.Println("Pod name is:", pod.Name)
	c.reporter.Report(pod)
}

func tweakListOpts(opts *metav1.ListOptions) {
	opts.LabelSelector = fmt.Sprintf(
		"%s in (%s)",
		k8s.LabelSourceType, "STG",
	)
}
