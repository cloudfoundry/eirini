package jobs_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/jobs/jobsfakes"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatusGetter", func() {
	var (
		statusGetter jobs.StatusGetter
		jobGetter    *jobsfakes.FakeJobGetter
	)

	BeforeEach(func() {
		jobGetter = new(jobsfakes.FakeJobGetter)
		jobGetter.GetByGUIDReturns([]batchv1.Job{{}}, nil)
		statusGetter = jobs.NewStatusGetter(lagertest.NewTestLogger("status_getter_test"), jobGetter)
	})

	It("gets the task status", func() {
		status, err := statusGetter.GetStatus(context.Background(), "john")
		Expect(err).NotTo(HaveOccurred())

		Expect(status).To(Equal(eiriniv1.TaskStatus{ExecutionStatus: eiriniv1.TaskStarting}))
	})

	When("getting the job list fails", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns(nil, errors.New("boom"))
		})

		It("returns an error", func() {
			_, err := statusGetter.GetStatus(context.Background(), "john")
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})
	})

	When("getting the job list returns no jobs", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns([]batchv1.Job{}, nil)
		})

		It("returns an error", func() {
			_, err := statusGetter.GetStatus(context.Background(), "john")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	When("getting the job list returns multiple jobs", func() {
		BeforeEach(func() {
			jobGetter.GetByGUIDReturns([]batchv1.Job{{}, {}}, nil)
		})

		It("returns an error", func() {
			_, err := statusGetter.GetStatus(context.Background(), "john")
			Expect(err).To(MatchError(ContainSubstring("multiple jobs found for task")))
		})
	})

	When("the job is running", func() {
		var now metav1.Time

		BeforeEach(func() {
			now = metav1.Now()
			job := batchv1.Job{
				Status: batchv1.JobStatus{
					StartTime: &now,
				},
			}
			jobGetter.GetByGUIDReturns([]batchv1.Job{job}, nil)
		})

		It("returns a running status", func() {
			status, err := statusGetter.GetStatus(context.Background(), "john")
			Expect(err).NotTo(HaveOccurred())

			Expect(status).To(Equal(eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskRunning,
				StartTime:       &now,
			}))
		})
	})

	When("the job has succeeded", func() {
		var (
			now   metav1.Time
			later metav1.Time
		)

		BeforeEach(func() {
			now = metav1.Now()
			later = metav1.NewTime(now.Add(time.Hour))
			job := batchv1.Job{
				Status: batchv1.JobStatus{
					StartTime:      &now,
					Succeeded:      1,
					CompletionTime: &later,
				},
			}
			jobGetter.GetByGUIDReturns([]batchv1.Job{job}, nil)
		})

		It("returns a succeeded status", func() {
			status, err := statusGetter.GetStatus(context.Background(), "john")
			Expect(err).NotTo(HaveOccurred())

			Expect(status).To(Equal(eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskSucceeded,
				StartTime:       &now,
				EndTime:         &later,
			}))
		})
	})

	When("the job has failed", func() {
		var (
			now   metav1.Time
			later metav1.Time
		)

		BeforeEach(func() {
			now = metav1.Now()
			later = metav1.NewTime(now.Add(time.Hour))
			job := batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:               batchv1.JobComplete,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               batchv1.JobFailed,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               batchv1.JobFailed,
							LastTransitionTime: later,
						},
					},
					StartTime: &now,
					Failed:    1,
				},
			}
			jobGetter.GetByGUIDReturns([]batchv1.Job{job}, nil)
		})

		It("returns a failed status", func() {
			status, err := statusGetter.GetStatus(context.Background(), "john")
			Expect(err).NotTo(HaveOccurred())

			Expect(status).To(Equal(eiriniv1.TaskStatus{
				ExecutionStatus: eiriniv1.TaskFailed,
				StartTime:       &now,
				EndTime:         &later,
			}))
		})
	})
})
