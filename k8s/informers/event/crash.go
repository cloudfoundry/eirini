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
		workChan:    make(chan v1.Pod, 20),
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
	mObj := newObj.(*v1.Pod)
	c.workChan <- *mObj
}

func (c *CrashInformer) Work() {
	for {
		select {
		case pod := <-c.workChan:
			statuses := pod.Status.ContainerStatuses
			if statuses == nil || len(statuses) == 0 {
				continue
			}

			terminated := pod.Status.ContainerStatuses[0].State.Terminated
			if terminated != nil && terminated.ExitCode != 0 {
				c.reportState(pod, terminated.Reason, int(terminated.ExitCode), terminated.Reason, int64(terminated.StartedAt.Second()))
				continue
			}

			waiting := pod.Status.ContainerStatuses[0].State.Waiting
			if waiting != nil && waiting.Reason == CrashLoopBackOff {
				container := pod.Status.ContainerStatuses[0]
				exitStatus := int(container.LastTerminationState.Terminated.ExitCode)
				exitDescription := container.LastTerminationState.Terminated.Reason
				crashTimestamp := int64(container.LastTerminationState.Terminated.StartedAt.Second())
				c.reportState(pod, waiting.Reason, exitStatus, exitDescription, crashTimestamp)
			}

		case <-c.stopperChan:
			return
		}
	}
}

func (c *CrashInformer) reportState(
	pod v1.Pod,
	reason string,
	exitStatus int,
	exitDescription string,
	crashTimestamp int64,
) {
	if report, err := toReport(pod, reason, exitStatus, exitDescription, crashTimestamp); err == nil {
		c.reportChan <- report
	}
}

func toReport(
	pod v1.Pod,
	reason string,
	exitStatus int,
	exitDescription string,
	crashTimestamp int64,
) (events.CrashReport, error) {
	container := pod.Status.ContainerStatuses[0]
	index, err := parsePodIndex(pod.Name)
	if err != nil {
		return events.CrashReport{}, err
	}

	return events.CrashReport{
		ProcessGUID: pod.Annotations[cf.ProcessGUID],
		AppCrashedRequest: cc_messages.AppCrashedRequest{
			Reason:          reason,
			Instance:        pod.Name,
			Index:           index,
			ExitStatus:      exitStatus,
			ExitDescription: exitDescription,
			CrashTimestamp:  crashTimestamp,
			CrashCount:      int(container.RestartCount),
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
