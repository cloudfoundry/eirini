package event_test

import (
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
		informerStopper chan struct{}
		watcher         *watch.FakeWatcher
		logger          *lagertest.TestLogger

		eventGenerator *eventfakes.FakeCrashEventGenerator
		crashEmitter   *eventfakes.FakeCrashEmitter
	)

	BeforeEach(func() {
		eventGenerator = new(eventfakes.FakeCrashEventGenerator)
		crashEmitter = new(eventfakes.FakeCrashEmitter)
		informerStopper = make(chan struct{})

		logger = lagertest.NewTestLogger("crash-event-logger-test")
		client = fake.NewSimpleClientset()

		watcher = watch.NewFake()
		client.PrependWatchReactor("pods", testing.DefaultWatchReactor(watcher, nil))
		crashInformer := NewCrashInformer(
			client,
			0,
			"namespace",
			informerStopper,
			logger,
			eventGenerator,
			crashEmitter,
		)

		go crashInformer.Start()
	})

	AfterEach(func() {
		close(informerStopper)
	})

	Context("When app does not have to be reported", func() {
		var event events.CrashEvent
		BeforeEach(func() {
			event = events.CrashEvent{
				ProcessGUID: "blahblah",
			}
			eventGenerator.GenerateReturns(event, false)

			loopy := &v1.Pod{}
			watcher.Add(loopy)
			watcher.Modify(loopy)
		})

		It("should NOT send a crash event", func() {
			Expect(crashEmitter.EmitCallCount()).To(Equal(0))
		})
	})

	Context("When app has to be reported", func() {
		var (
			event events.CrashEvent
			pod   *v1.Pod
		)
		BeforeEach(func() {
			event = events.CrashEvent{
				ProcessGUID: "blahblah",
			}
			eventGenerator.GenerateReturns(event, true)

			pod = &v1.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name: "i-am-test-pod",
				},
			}
			watcher.Add(pod)
			watcher.Modify(pod)
		})

		It("sends correct args to the event generator", func() {
			Eventually(eventGenerator.GenerateCallCount).Should(Equal(1))
			inputPod, inputLogger := eventGenerator.GenerateArgsForCall(0)
			Expect(inputPod).To(Equal(pod))
			Expect(inputLogger).To(Equal(logger))
		})

		It("should send a crash event", func() {
			Eventually(crashEmitter.EmitCallCount).Should(Equal(1))

			actualevent := crashEmitter.EmitArgsForCall(0)
			Expect(actualevent.ProcessGUID).To(Equal("blahblah"))
		})
	})
})
