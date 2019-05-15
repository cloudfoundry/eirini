package wait_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/rootfspatcher/wait"
	"code.cloudfoundry.org/eirini/rootfspatcher/wait/waitfakes"
	"code.cloudfoundry.org/lager/lagertest"
	apicore "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Waiter", func() {

	var (
		podsRunning   PodsRunning
		logger        *lagertest.TestLogger
		fakePodLister *waitfakes.FakePodLister
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakePodLister = new(waitfakes.FakePodLister)

		podsRunning = PodsRunning{
			PodLister: fakePodLister,
			Logger:    logger,
			Timeout:   1 * time.Second,
			Label:     "my=label",
		}
	})

	Context("When all pods are running", func() {
		It("should finish", func() {
			channel := make(chan error, 1)
			defer close(channel)

			fakePodLister.ListReturns(createPodReady(), nil)

			go func(ch chan<- error) {
				err := podsRunning.Wait()
				ch <- err
			}(channel)

			Eventually(channel).Should(Receive(BeNil()))
			Expect(fakePodLister.ListCallCount()).To(Equal(1))
		})
	})

	Context("When pods are not coming up", func() {
		It("should time out", func() {
			channel := make(chan error, 1)
			defer close(channel)

			fakePodLister.ListReturns(createPodUnready(), nil)

			go func(ch chan<- error) {
				err := podsRunning.Wait()
				ch <- err
			}(channel)

			Eventually(channel, "2s").Should(Receive(MatchError("timed out after 1s")))
			Expect(fakePodLister.ListCallCount()).To(BeNumerically(">", 0))
		})
	})

	Context("When the specified timeout is not valid", func() {
		It("should return an error", func() {
			podsRunning.Timeout = -1
			err := podsRunning.Wait()
			Expect(err).To(MatchError("provided timeout is not valid"))
		})

	})

	Context("When listing pods fails", func() {
		It("should log something", func() {
			fakePodLister.ListReturns(nil, errors.New("boom?"))

			err := podsRunning.Wait()
			Expect(err).To(HaveOccurred())
			Expect(logger.LogMessages()).To(ContainElement("test.failed to list pods"))
			Expect(logger.Logs()[0].Data["error"]).To(Equal("boom?"))

		})
	})

	Context("When listing pods", func() {
		It("should use the right label selector", func() {
			fakePodLister.ListReturns(createPodReady(), nil)

			_ = podsRunning.Wait()
			listOptions := fakePodLister.ListArgsForCall(0)
			Expect(listOptions.LabelSelector).To(Equal("my=label"))
		})
	})
})

func createPodReady() *apicore.PodList {
	return createPod(true)
}

func createPodUnready() *apicore.PodList {
	return createPod(false)
}

func createPod(ready bool) *apicore.PodList {
	return &apicore.PodList{
		Items: []apicore.Pod{
			{
				ObjectMeta: meta.ObjectMeta{
					Name: "test-pod",
				},
				Status: apicore.PodStatus{
					ContainerStatuses: []apicore.ContainerStatus{{
						Ready: ready,
					}},
				},
			},
		},
	}
}
