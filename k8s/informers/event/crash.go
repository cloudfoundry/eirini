package event

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const CrashLoopBackOff = "CrashLoopBackOff"

type CrashInformer struct {
	clientset   kubernetes.Interface
	syncPeriod  time.Duration
	namespace   string
	reportChan  chan events.CrashReport
	stopperChan chan struct{}
	workChan    chan v1.Pod
}

func NewCrashInformer(
	client kubernetes.Interface,
	syncPeriod time.Duration,
	namespace string,
	reportChan chan events.CrashReport,
	stopperChan chan struct{},
) *CrashInformer {
	return &CrashInformer{
		clientset:   client,
		syncPeriod:  syncPeriod,
		namespace:   namespace,
		reportChan:  reportChan,
		stopperChan: stopperChan,
		workChan:    make(chan v1.Pod),
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

func (c *CrashInformer) updateFunc(obj interface{}, newObj interface{}) {
	mObj := obj.(*v1.Pod)
	c.workChan <- *mObj
}

func (c *CrashInformer) Work() {
	for {
		select {
		case pod := <-c.workChan:
			waiting := pod.Status.ContainerStatuses[0].State.Waiting
			if waiting != nil && waiting.Reason == CrashLoopBackOff {
				report, err := toReport(pod)
				if err != nil {
					continue
				}
				c.reportChan <- report
			}
		case <-c.stopperChan:
			return
		}
	}
}

func toReport(pod v1.Pod) (events.CrashReport, error) {
	container := pod.Status.ContainerStatuses[0]
	index, err := parsePodIndex(pod.Name)
	if err != nil {
		return events.CrashReport{}, err
	}

	return events.CrashReport{
		ProcessGuid: pod.Annotations[cf.ProcessGUID],
		AppCrashedRequest: cc_messages.AppCrashedRequest{
			Reason:          container.State.Waiting.Reason,
			Instance:        pod.Name,
			Index:           index,
			ExitStatus:      int(container.LastTerminationState.Terminated.ExitCode),
			ExitDescription: container.LastTerminationState.Terminated.Reason,
			CrashCount:      int(container.RestartCount),
			CrashTimestamp:  int64(container.LastTerminationState.Terminated.FinishedAt.Second()),
		},
	}, nil
}

func parsePodIndex(podName string) (int, error) {
	sl := strings.Split(podName, "-")

	if len(sl) <= 1 {
		return 0, fmt.Errorf("Could not parse pod name from %s", podName)
	}
	return strconv.Atoi(sl[len(sl)-1])
}
