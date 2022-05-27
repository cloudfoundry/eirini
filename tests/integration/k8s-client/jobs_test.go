package integration_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Jobs", func() {
	var jobsClient *client.Job

	BeforeEach(func() {
		jobsClient = client.NewJob(fixture.Clientset, "")
	})

	Describe("Create", func() {
		It("creates a Job", func() {
			runAsNonRoot := true
			runAsUser := int64(2000)
			_, err := jobsClient.Create(ctx, fixture.Namespace, &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: &runAsNonRoot,
								RunAsUser:    &runAsUser,
							},
							Containers: []corev1.Container{
								{
									Name:            "test",
									Image:           "eirini/busybox",
									ImagePullPolicy: corev1.PullAlways,
									Command:         []string{"echo", "hi"},
								},
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			jobs := listJobs(fixture.Namespace)

			Expect(jobs).To(HaveLen(1))
			Expect(jobs[0].Name).To(Equal("foo"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createJob(fixture.Namespace, "foo", nil)
		})

		It("deletes a Job", func() {
			Eventually(func() []batchv1.Job { return listJobs(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := jobsClient.Delete(ctx, fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []batchv1.Job { return listJobs(fixture.Namespace) }).Should(BeEmpty())
		})
	})

	Describe("GetByGUID", func() {
		BeforeEach(func() {
			createJob(fixture.Namespace, "foo", map[string]string{
				jobs.LabelGUID: "bar",
			})

			createJob(fixture.Namespace, "foo-complete", map[string]string{
				jobs.LabelGUID:          "baz",
				jobs.LabelTaskCompleted: jobs.TaskCompletedTrue,
			})

			extraNs := fixture.CreateExtraNamespace()
			createJob(extraNs, "foo2", map[string]string{
				jobs.LabelGUID: "bar",
			})
		})

		getJobGUIDs := func(guid string, includeCompleted bool) func() []string {
			return func() []string {
				jobs, err := jobsClient.GetByGUID(ctx, guid, includeCompleted)
				Expect(err).NotTo(HaveOccurred())

				return jobNames(jobs)
			}
		}

		When("not including completed jobs", func() {
			It("gets all jobs not labelled as completed matching the specified guid", func() {
				Eventually(getJobGUIDs("bar", false)).Should(ContainElements("foo", "foo2"))
				Consistently(getJobGUIDs("baz", false)).ShouldNot(ContainElement("foo-complete"))
			})
		})

		When("including completed jobs", func() {
			It("gets a job labelled as completed", func() {
				Eventually(getJobGUIDs("baz", true)).Should(ContainElement("foo-complete"))
			})
		})
	})

	Describe("List", func() {
		var (
			taskGUID          string
			extraTaskGUID     string
			completedTaskGUID string
			extraNs           string
		)

		BeforeEach(func() {
			taskGUID = tests.GenerateGUID()
			extraTaskGUID = tests.GenerateGUID()
			completedTaskGUID = tests.GenerateGUID()
			extraNs = fixture.CreateExtraNamespace()

			createJob(fixture.Namespace, "foo", map[string]string{
				jobs.LabelGUID:       taskGUID,
				jobs.LabelSourceType: "TASK",
			})
			createJob(fixture.Namespace, "completedfoo", map[string]string{
				jobs.LabelGUID:          completedTaskGUID,
				jobs.LabelSourceType:    "TASK",
				jobs.LabelTaskCompleted: "true",
			})
			createJob(extraNs, "bas", map[string]string{
				jobs.LabelGUID:       extraTaskGUID,
				jobs.LabelSourceType: "TASK",
			})
		})

		listJobGUIDs := func(includeCompleted bool) func() []string {
			return func() []string {
				jobs, err := jobsClient.List(ctx, includeCompleted)
				Expect(err).NotTo(HaveOccurred())

				return jobGUIDs(jobs)
			}
		}

		When("including completed tasks", func() {
			It("lists all task jobs", func() {
				Eventually(listJobGUIDs(true)).Should(ContainElements(taskGUID, extraTaskGUID, completedTaskGUID))
			})
		})

		When("excluding completed tasks", func() {
			It("does not list completed tasks", func() {
				Consistently(listJobGUIDs(false)).ShouldNot(ContainElements(completedTaskGUID))
			})
		})

		When("the workloads namespace is set", func() {
			BeforeEach(func() {
				jobsClient = client.NewJob(fixture.Clientset, fixture.Namespace)
			})

			It("lists task jobs from the workloads namespace only", func() {
				Eventually(listJobGUIDs(true)).Should(ContainElements(taskGUID, completedTaskGUID))
			})
		})
	})

	Describe("SetAnnotation", func() {
		BeforeEach(func() {
			createJob(fixture.Namespace, "foo", map[string]string{
				jobs.LabelGUID:       "the-task-guid",
				jobs.LabelSourceType: "TASK",
			})
		})

		It("sets the annotation", func() {
			Eventually(func() []batchv1.Job { return listJobs(fixture.Namespace) }).ShouldNot(BeEmpty())

			job, err := getJob("the-task-guid")
			Expect(err).NotTo(HaveOccurred())

			_, err = jobsClient.SetAnnotation(ctx, job, "foo", "bar")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				job, err := getJob("the-task-guid")
				Expect(err).NotTo(HaveOccurred())

				return job.Annotations["foo"]
			}).Should(Equal("bar"))
		})
	})
	Describe("SetLabel", func() {
		var (
			taskGUID       string
			label          string
			value          string
			oldJob, newJob *batchv1.Job
			err            error
		)

		BeforeEach(func() {
			taskGUID = tests.GenerateGUID()
			createJob(fixture.Namespace, "foo", map[string]string{
				jobs.LabelGUID:       taskGUID,
				jobs.LabelSourceType: "TASK",
			})

			oldJob, err = getJob(taskGUID)
			Expect(err).NotTo(HaveOccurred())

			label = "foo"
			value = "bar"
		})

		JustBeforeEach(func() {
			newJob, err = jobsClient.SetLabel(ctx, oldJob, label, value)
		})

		It("adds the label", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(newJob.Labels).To(HaveKeyWithValue("foo", "bar"))
		})

		It("preserves old labels", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(newJob.Labels).To(HaveKeyWithValue(jobs.LabelGUID, taskGUID))
			Expect(newJob.Labels).To(HaveKeyWithValue(jobs.LabelSourceType, "TASK"))
		})

		When("setting an existing label", func() {
			BeforeEach(func() {
				label = jobs.LabelSourceType
				value = "APP"
			})

			It("replaces the label", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newJob.Labels).To(HaveKeyWithValue(jobs.LabelSourceType, "APP"))
			})
		})

		When("job is updated between getting and setting", func() {
			BeforeEach(func() {
				_, err = jobsClient.SetLabel(ctx, oldJob, "foo", "something-else")
				Expect(err).NotTo(HaveOccurred())
			})

			It("overwrites the changed value with the new value", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newJob.Labels).To(HaveKeyWithValue("foo", "bar"))
			})
		})
	})
})

func jobNames(jobs []batchv1.Job) []string {
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		names = append(names, job.Name)
	}

	return names
}

func jobGUIDs(allJobs []batchv1.Job) []string {
	guids := make([]string, 0, len(allJobs))
	for _, job := range allJobs {
		guids = append(guids, job.Labels[jobs.LabelGUID])
	}

	return guids
}

func getJob(taskGUID string) (*batchv1.Job, error) {
	jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", jobs.LabelGUID, taskGUID),
	})
	if err != nil {
		return nil, err
	}

	if len(jobs.Items) != 1 {
		return nil, fmt.Errorf("expected 1 job with guid %s, got %d", taskGUID, len(jobs.Items))
	}

	return &jobs.Items[0], nil
}
