package event_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/events"
	. "code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("Event", func() {

	var (
		client        kubernetes.Interface
		syncPeriod    time.Duration
		namespace     string
		crashInformer *CrashInformer

		reportChan      chan events.CrashReport
		informerStopper chan struct{}

		watcher               *watch.FakeWatcher
		pinky, brain, bandito *v1.Pod

		crashTime meta.Time
	)

	BeforeEach(func() {
		reportChan = make(chan events.CrashReport)
		informerStopper = make(chan struct{})

		client = fake.NewSimpleClientset()
		syncPeriod = 0
		namespace = "milkyway"
		crashInformer = NewCrashInformer(client, syncPeriod, namespace, reportChan, informerStopper)

		watcher = watch.NewFake()
		fakecs := client.(*fake.Clientset)
		fakecs.PrependWatchReactor("pods", testing.DefaultWatchReactor(watcher, nil))

		pinky = createPod("pinky-pod", 0)
		brain = createPod("brain-pod", 0)
		bandito = createStatelessPod("bandito")
	})

	AfterEach(func() {
		close(informerStopper)
	})

	JustBeforeEach(func() {
		go crashInformer.Start()
		go crashInformer.Work()

		watcher.Add(pinky)
		watcher.Add(brain)
		watcher.Add(bandito)
	})

	Context("When an app crashes", func() {
		JustBeforeEach(func() {
			pinky.Status.ContainerStatuses[0].State = v1.ContainerState{
				Waiting: &v1.ContainerStateWaiting{
					Reason: CrashLoopBackOff,
				},
			}
			crashTime = meta.Time{time.Now()}
			pinky.Status.ContainerStatuses[0].LastTerminationState = v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					ExitCode:   -1,
					Reason:     "this describes how much you screwed up",
					FinishedAt: crashTime,
				},
			}
			pinky.Status.ContainerStatuses[0].RestartCount = 3

			brain.Status.ContainerStatuses[0].State = v1.ContainerState{
				Waiting: &v1.ContainerStateWaiting{
					Reason: "sleepy",
				},
			}

			bandito.Name = "no-bandito"
			watcher.Modify(pinky)
			watcher.Modify(brain)
			watcher.Modify(bandito)
		})

		It("should send reports the report chan", func() {
			Eventually(reportChan).Should(Receive())
		})

		It("should receive a crashed report", func() {
			Eventually(reportChan).Should(Receive(Equal(events.CrashReport{
				ProcessGuid: "pinky-pod-anno",
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Reason:          CrashLoopBackOff,
					Instance:        "pinky-pod-0",
					Index:           0,
					ExitStatus:      -1,
					ExitDescription: "this describes how much you screwed up",
					CrashCount:      3,
					CrashTimestamp:  int64(crashTime.Time.Second()),
				},
			})))
		})

		It("should not get more reports", func() {
			<-reportChan
			Consistently(reportChan).ShouldNot(Receive())
		})
	})
})

func createPod(name string, index int) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%s-%d", name, index),
			Annotations: map[string]string{
				cf.ProcessGUID: fmt.Sprintf("%s-anno", name),
			},
			OwnerReferences: []meta.OwnerReference{
				{
					Kind: "StatefulSet",
					Name: "mr-stateful",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}

func createStatelessPod(name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}
