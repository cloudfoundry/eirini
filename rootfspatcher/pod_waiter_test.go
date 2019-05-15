package rootfspatcher_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/rootfspatcher/rootfspatcherfakes"
	"code.cloudfoundry.org/lager/lagertest"
	apicore "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Waiter", func() {

	var (
		podWaiter     PodWaiter
		logger        *lagertest.TestLogger
		fakePodLister *rootfspatcherfakes.FakePodLister
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakePodLister = new(rootfspatcherfakes.FakePodLister)

		podWaiter = PodWaiter{
			PodLister:        fakePodLister,
			Logger:           logger,
			Timeout:          1 * time.Second,
			PodLabelSelector: "my=label",
		}
	})

	Context("When all pods are running", func() {
		It("should finish", func() {
			channel := make(chan error, 1)
			defer close(channel)

			fakePodLister.ListReturns(createPodReady(), nil)
			err := podWaiter.Wait()

			Expect(err).ToNot(HaveOccurred())
			Expect(fakePodLister.ListCallCount()).To(Equal(1))
		})
	})

	Context("When pods are not coming up", func() {
		It("should time out", func() {
			channel := make(chan error, 1)
			defer close(channel)

			fakePodLister.ListReturns(createPodUnready(), nil)

			Expect(podWaiter.Wait()).To(MatchError("timed out after 1s"))
			Expect(fakePodLister.ListCallCount()).To(BeNumerically(">", 0))
		})
	})

	Context("When pods don't match the expected labels", func() {
		Context("key is missing", func() {
			BeforeEach(func() {
				podWaiter.ExpectedPodLabels = map[string]string{
					"foo": "bar",
					"baz": "wat",
				}

				podList := createPodReady()
				podList.Items[0].Labels = map[string]string{
					"foo": "bar",
				}
				fakePodLister.ListReturns(podList, nil)
			})

			It("should time out", func() {
				channel := make(chan error, 1)
				defer close(channel)

				go func(ch chan<- error) {
					err := podWaiter.Wait()
					ch <- err
				}(channel)

				Eventually(channel, "2s").Should(Receive(MatchError("timed out after 1s")))
				Expect(fakePodLister.ListCallCount()).To(BeNumerically(">", 0))
			})
		})

		Context("value is different", func() {
			BeforeEach(func() {
				podWaiter.ExpectedPodLabels = map[string]string{
					"foo": "bar",
					"baz": "wat",
				}

				podList := createPodReady()
				podList.Items[0].Labels = map[string]string{
					"foo": "bar",
					"baz": "bat",
				}
				fakePodLister.ListReturns(podList, nil)
			})

			It("should time out", func() {
				channel := make(chan error, 1)
				defer close(channel)

				go func(ch chan<- error) {
					err := podWaiter.Wait()
					ch <- err
				}(channel)

				Eventually(channel, "2s").Should(Receive(MatchError("timed out after 1s")))
				Expect(fakePodLister.ListCallCount()).To(BeNumerically(">", 0))
			})
		})
	})

	Context("When the specified timeout is not valid", func() {
		It("should return an error", func() {
			podWaiter.Timeout = -1
			err := podWaiter.Wait()
			Expect(err).To(MatchError("provided timeout is not valid"))
		})

	})

	Context("When listing pods fails", func() {
		It("should log the error", func() {
			fakePodLister.ListReturns(nil, errors.New("boom?"))

			err := podWaiter.Wait()
			Expect(err).To(HaveOccurred())
			Expect(logger.LogMessages()).To(ContainElement("test.failed to list pods"))
			Expect(logger.Logs()[0].Data["error"]).To(Equal("boom?"))

		})
	})

	Context("When listing pods", func() {
		It("should use the right label selector", func() {
			fakePodLister.ListReturns(createPodReady(), nil)

			_ = podWaiter.Wait()
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
