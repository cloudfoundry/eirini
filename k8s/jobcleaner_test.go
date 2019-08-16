package k8s_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
)

var _ = Describe("Jobcleaner", func() {

	var (
		cleaner Cleaner
		client  *k8sfakes.FakeJobDeleterClient
	)

	BeforeEach(func() {
		client = new(k8sfakes.FakeJobDeleterClient)
		jobs := &v1.JobList{
			Items: []v1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
			},
		}
		client.ListReturns(jobs, nil)

		cleaner = JobCleaner{Jobs: client}
	})

	It("selects the correct jobs to delete", func() {
		Expect(cleaner.Clean("guid=some-guid")).To(Succeed())
		Expect(client.ListCallCount()).To(Equal(1))
		listOpts := client.ListArgsForCall(0)
		Expect(listOpts.LabelSelector).To(Equal("guid=some-guid"))
	})

	It("deletes the jobs", func() {
		Expect(cleaner.Clean("guid=some-guid")).To(Succeed())
		Expect(client.DeleteCallCount()).To(Equal(1))

		jobName, opts := client.DeleteArgsForCall(0)
		Expect(jobName).To(Equal("foo"))
		Expect(*opts.PropagationPolicy).To(Equal(metav1.DeletePropagationBackground))
	})

	When("listing of jobs fails", func() {
		It("should return a meaningful error", func() {
			client.ListReturns(nil, errors.New("boom"))
			Expect(cleaner.Clean("guid=some-guid")).To(MatchError(ContainSubstring("failed to list jobs")))
		})
	})

	When("deletion of jobs fails", func() {
		It("should return a meaningful error", func() {
			client.DeleteReturns(errors.New("boom"))
			Expect(cleaner.Clean("guid=some-guid")).To(MatchError(ContainSubstring("failed to delete job")))
		})
	})
})
