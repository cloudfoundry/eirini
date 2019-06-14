package event_test

import (
	"fmt"
	"sync"

	"code.cloudfoundry.org/eirini/events"
	. "code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/eirini/k8s/informers/event/eventfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("Event", func() {

	var (
		client          *fake.Clientset
		reportChan      chan events.CrashReport
		informerStopper chan struct{}
		watcher         *watch.FakeWatcher
		logger          *lagertest.TestLogger
		informerWG      sync.WaitGroup

		reportGenerator *eventfakes.FakeCrashReportGenerator
	)

	BeforeEach(func() {
		reportGenerator = new(eventfakes.FakeCrashReportGenerator)
		reportChan = make(chan events.CrashReport)
		informerStopper = make(chan struct{})

		logger = lagertest.NewTestLogger("crash-event-logger-test")
		client = fake.NewSimpleClientset()

		watcher = watch.NewFake()
		client.PrependWatchReactor("pods", testing.DefaultWatchReactor(watcher, nil))
		informerWG = sync.WaitGroup{}
		informerWG.Add(1)
		crashInformer := NewCrashInformer(client, 0, "namespace", reportChan, informerStopper, logger, reportGenerator)
		go func() {
			crashInformer.Start()
			informerWG.Done()
		}()
	})

	AfterEach(func() {
		close(informerStopper)
		informerWG.Wait()
	})

	Context("When app does not have to be reported", func() {
		var report events.CrashReport
		BeforeEach(func() {
			report = events.CrashReport{
				ProcessGUID: "blahblah",
			}
			reportGenerator.GenerateReturns(report, false)

			loopy := &v1.Pod{}
			watcher.Add(loopy)
			watcher.Modify(loopy)
		})

		It("should receive a crashed report", func() {
			Consistently(reportChan).ShouldNot(Receive())
		})
	})

	Context("When app has to be reported", func() {
		var (
			report events.CrashReport
			pod    *v1.Pod
		)
		BeforeEach(func() {
			report = events.CrashReport{
				ProcessGUID: "blahblah",
			}
			reportGenerator.GenerateReturns(report, true)

			pod = &v1.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name: fmt.Sprintf("i-am-test-pod"),
				},
			}
			watcher.Add(pod)
			watcher.Modify(pod)
		})

		It("sends correct args to the report generator", func() {
			Eventually(reportChan).Should(Receive(Equal(report)))
			Expect(reportGenerator.GenerateCallCount()).To(Equal(1))
			inputPod, inputClient, inputLogger := reportGenerator.GenerateArgsForCall(0)
			Expect(inputPod).To(Equal(pod))
			Expect(inputClient).To(Equal(client))
			Expect(inputLogger).To(Equal(logger))

		})

		It("should receive a crashed report", func() {
			Eventually(reportChan).Should(Receive(Equal(report)))
		})
	})
})
