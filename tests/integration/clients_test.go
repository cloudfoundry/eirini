package integration_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Pod", func() {
	var podClient *client.Pod

	BeforeEach(func() {
		podClient = client.NewPod(fixture.Clientset, "")
	})

	Describe("GetAll", func() {
		var extraNs string

		BeforeEach(func() {
			extraNs = fixture.CreateExtraNamespace()

			createLrpPods(fixture.Namespace, "one", "two", "three")
			createTaskPods(extraNs, "four", "five", "six")
			createPod(extraNs, "sadpod", map[string]string{})
		})

		It("lists all eirini pods across all namespaces", func() {
			Eventually(func() []string {
				pods, err := podClient.GetAll()
				Expect(err).NotTo(HaveOccurred())

				return podNames(pods)
			}).Should(SatisfyAll(
				ContainElements("one", "two", "three", "four", "five", "six"),
				Not(ContainElement("sadpod")),
			))
		})

		When("the workloads namespace is set", func() {
			BeforeEach(func() {
				podClient = client.NewPod(fixture.Clientset, fixture.Namespace)
			})

			It("lists eirini pods from the configured namespace only", func() {
				Eventually(func() []string {
					pods, err := podClient.GetAll()
					Expect(err).NotTo(HaveOccurred())

					return podNames(pods)
				}).Should(SatisfyAll(
					ContainElements("one", "two", "three"),
					Not(ContainElements("four", "five", "six", "sadpod")),
				))
			})
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			createLrpPods(fixture.Namespace, "one", "two", "three")

			guid = tests.GenerateGUID()

			createPod(fixture.Namespace, "four", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})
			createPod(fixture.Namespace, "five", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createPod(extraNs, "six", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})
		})

		It("lists all pods matching the specified LRP identifier", func() {
			pods, err := podClient.GetByLRPIdentifier(opi.LRPIdentifier{GUID: guid, Version: "42"})

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return podNames(pods) }).Should(ConsistOf("four", "five", "six"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createLrpPods(fixture.Namespace, "foo")
		})

		It("deletes a pod", func() {
			Eventually(func() []string { return podNames(listAllPods()) }).Should(ContainElement("foo"))

			err := podClient.Delete(fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return podNames(listAllPods()) }).ShouldNot(ContainElement("foo"))
		})

		Context("when it fails", func() {
			It("returns the error", func() {
				err := podClient.Delete(fixture.Namespace, "bar")

				Expect(err).To(MatchError(ContainSubstring(`"bar" not found`)))
			})
		})
	})

	Describe("SetAnnotation", func() {
		var (
			key            string
			value          string
			oldPod, newPod *corev1.Pod
			err            error
		)

		BeforeEach(func() {
			key = "foo"
			value = "bar"

			createLrpPods(fixture.Namespace, "foo")
			oldPod = getPod(fixture.Namespace, "foo")
		})

		JustBeforeEach(func() {
			newPod, err = podClient.SetAnnotation(oldPod, key, value)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets a pod annotation", func() {
			Expect(newPod.Annotations["foo"]).To(Equal("bar"))
		})

		It("preserves existing annotations", func() {
			Expect(newPod.Annotations["some"]).To(Equal("annotation"))
		})

		When("setting an existing annotation", func() {
			BeforeEach(func() {
				key = "some"
			})

			It("overrides that annotation", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newPod.Annotations["some"]).To(Equal("bar"))
			})
		})

		When("pod was updated since being read", func() {
			BeforeEach(func() {
				_, err = podClient.SetAnnotation(oldPod, "foo", "something-else")
				Expect(err).NotTo(HaveOccurred())
			})

			It("overwrites the change without failing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newPod.Annotations["foo"]).To(Equal("bar"))
			})
		})
	})

	Describe("SetAndTestAnnotation", func() {
		var (
			key            string
			value          string
			prevValue      *string
			oldPod, newPod *corev1.Pod
			err            error
		)

		BeforeEach(func() {
			key = "foo"
			value = "bar"
			prevValue = nil

			createLrpPods(fixture.Namespace, "foo")
			oldPod = getPod(fixture.Namespace, "foo")
		})

		JustBeforeEach(func() {
			newPod, err = podClient.SetAndTestAnnotation(oldPod, key, value, prevValue)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets a pod annotation", func() {
			Expect(newPod.Annotations["foo"]).To(Equal("bar"))
		})

		It("preserves existing annotations", func() {
			Expect(newPod.Annotations["some"]).To(Equal("annotation"))
		})

		When("setting an existing annotation", func() {
			BeforeEach(func() {
				key = "some"
				prevValueStr := "annotation"
				prevValue = &prevValueStr
			})

			It("overrides that annotation", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newPod.Annotations["some"]).To(Equal("bar"))
			})
		})

		When("the previous value doesn't match that supplied", func() {
			BeforeEach(func() {
				key = "some"
				prevValueStr := "notTheValue"
				prevValue = &prevValueStr
			})

			It("fails", func() {
				Expect(err).To(MatchError(ContainSubstring("the server rejected")))
			})
		})
	})
})

var _ = Describe("PodDisruptionBudgets", func() {
	var pdbClient *client.PodDisruptionBudget

	BeforeEach(func() {
		pdbClient = client.NewPodDisruptionBudget(fixture.Clientset)
	})

	Describe("Create", func() {
		It("creates a PDB", func() {
			_, err := pdbClient.Create(fixture.Namespace, &policyv1beta1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			pdbs := listPDBs(fixture.Namespace)

			Expect(pdbs).To(HaveLen(1))
			Expect(pdbs[0].Name).To(Equal("foo"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createPDB(fixture.Namespace, "foo")
		})

		It("deletes a PDB", func() {
			Eventually(func() []policyv1beta1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := pdbClient.Delete(fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []policyv1beta1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).Should(BeEmpty())
		})
	})
})

var _ = Describe("StatefulSets", func() {
	var statefulSetClient *client.StatefulSet

	BeforeEach(func() {
		statefulSetClient = client.NewStatefulSet(fixture.Clientset, "")
	})

	Describe("Create", func() {
		It("creates a StatefulSet", func() {
			_, err := statefulSetClient.Create(fixture.Namespace, &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "foo",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "foo",
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			statefulSets := listStatefulSets(fixture.Namespace)

			Expect(statefulSets).To(HaveLen(1))
			Expect(statefulSets[0].Name).To(Equal("foo"))
		})
	})

	Describe("Get", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = tests.GenerateGUID()

			createStatefulSet(fixture.Namespace, "foo", map[string]string{
				stset.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "foo", nil)
		})

		It("retrieves a StatefulSet by namespace and name", func() {
			statefulSet, err := statefulSetClient.Get(fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(statefulSet.Name).To(Equal("foo"))
			Expect(statefulSet.Labels[stset.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("GetBySourceType", func() {
		var extraNs string

		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "one", map[string]string{
				stset.LabelSourceType: "FOO",
			})
			createStatefulSet(fixture.Namespace, "two", map[string]string{
				stset.LabelSourceType: "BAR",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "three", map[string]string{
				stset.LabelSourceType: "FOO",
			})
		})

		It("lists all StatefulSets with the specified source type", func() {
			Eventually(func() []string {
				statefulSets, err := statefulSetClient.GetBySourceType("FOO")
				Expect(err).NotTo(HaveOccurred())

				return statefulSetNames(statefulSets)
			}).Should(ContainElements("one", "three"))

			Consistently(func() []string {
				statefulSets, err := statefulSetClient.GetBySourceType("FOO")
				Expect(err).NotTo(HaveOccurred())

				return statefulSetNames(statefulSets)
			}).ShouldNot(ContainElements("two"))
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = tests.GenerateGUID()

			createStatefulSet(fixture.Namespace, "one", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "two", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})
		})

		It("lists all StatefulSets matching the specified LRP identifier", func() {
			statefulSets, err := statefulSetClient.GetByLRPIdentifier(opi.LRPIdentifier{GUID: guid, Version: "42"})

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return statefulSetNames(statefulSets) }).Should(ConsistOf("one", "two"))
		})
	})

	Describe("Update", func() {
		var statefulSet *appsv1.StatefulSet

		BeforeEach(func() {
			statefulSet = createStatefulSet(fixture.Namespace, "foo", map[string]string{
				"label": "old-value",
			})
		})

		It("updates a StatefulSet", func() {
			statefulSet.Labels["label"] = "new-value"

			newStatefulSet, err := statefulSetClient.Update(fixture.Namespace, statefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(newStatefulSet.Labels["label"]).To(Equal("new-value"))

			Eventually(func() string {
				return getStatefulSet(fixture.Namespace, "foo").Labels["label"]
			}).Should(Equal("new-value"))
		})
	})

	Describe("SetCPURequest", func() {
		var (
			statefulSet    *appsv1.StatefulSet
			containers     []corev1.Container
			cpuRequest     resource.Quantity
			newStatefulSet *appsv1.StatefulSet
		)

		BeforeEach(func() {
			cpuRequest = resource.MustParse("321m")

			containers = []corev1.Container{
				{
					Name:    "not-opi",
					Image:   "eirini/busybox",
					Command: []string{"echo", "hi"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("120m"),
						},
					},
				},
				{
					Name:    stset.OPIContainerName,
					Image:   "eirini/busybox",
					Command: []string{"echo", "hi"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("120m"),
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			statefulSet = createStatefulSetWithContainers(fixture.Namespace, "foo", containers)

			var err error
			newStatefulSet, err = statefulSetClient.SetCPURequest(statefulSet, &cpuRequest)
			Expect(err).NotTo(HaveOccurred())
		})

		getCPURequests := func(stSet *appsv1.StatefulSet) []int64 {
			millis := []int64{}

			for _, c := range stSet.Spec.Template.Spec.Containers {
				q := c.Resources.Requests[corev1.ResourceCPU]
				millis = append(millis, (&q).MilliValue())
			}

			return millis
		}

		It("patches CPU request onto an OPI container only on a StatefulSet", func() {
			Expect(getCPURequests(newStatefulSet)).To(Equal([]int64{120, 321}))

			Eventually(func() []int64 {
				stSet := getStatefulSet(fixture.Namespace, "foo")

				return getCPURequests(stSet)
			}).Should(Equal([]int64{120, 321}))
		})

		When("the stateful set doesn't have an opi container", func() {
			BeforeEach(func() {
				containers = []corev1.Container{
					{
						Name:    "not-opi",
						Image:   "eirini/busybox",
						Command: []string{"echo", "hi"},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("120m"),
							},
						},
					},
				}
			})

			It("does not modify cpu requests", func() {
				Expect(getCPURequests(newStatefulSet)).To(Equal([]int64{120}))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "foo", nil)
		})

		It("deletes a StatefulSet", func() {
			Eventually(func() []appsv1.StatefulSet { return listStatefulSets(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := statefulSetClient.Delete(fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []appsv1.StatefulSet { return listStatefulSets(fixture.Namespace) }).Should(BeEmpty())
		})
	})
})

var _ = Describe("Jobs", func() {
	var jobsClient *client.Job

	BeforeEach(func() {
		jobsClient = client.NewJob(fixture.Clientset, "")
	})

	Describe("Create", func() {
		It("creates a Job", func() {
			runAsNonRoot := true
			runAsUser := int64(2000)
			_, err := jobsClient.Create(fixture.Namespace, &batchv1.Job{
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

			err := jobsClient.Delete(fixture.Namespace, "foo")

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
				jobs, err := jobsClient.GetByGUID(guid, includeCompleted)
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
				jobs, err := jobsClient.List(includeCompleted)
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
			newJob, err = jobsClient.SetLabel(oldJob, label, value)
		})

		It("adds the foo label", func() {
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
				_, err = jobsClient.SetLabel(oldJob, "foo", "something-else")
				Expect(err).NotTo(HaveOccurred())
			})

			It("overwrites the changed value with the new value", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newJob.Labels).To(HaveKeyWithValue("foo", "bar"))
			})
		})
	})
})

var _ = Describe("Secrets", func() {
	var secretClient *client.Secret

	BeforeEach(func() {
		secretClient = client.NewSecret(fixture.Clientset)
	})

	Describe("Get", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = tests.GenerateGUID()

			createSecret(fixture.Namespace, "foo", map[string]string{
				stset.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createSecret(extraNs, "foo", nil)
		})

		It("retrieves a Secret by namespace and name", func() {
			secret, err := secretClient.Get(fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(secret.Name).To(Equal("foo"))
			Expect(secret.Labels[stset.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("Create", func() {
		It("creates the secret in the namespace", func() {
			_, createErr := secretClient.Create(fixture.Namespace, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "very-secret",
				},
			})
			Expect(createErr).NotTo(HaveOccurred())

			secrets := listSecrets(fixture.Namespace)
			Expect(secretNames(secrets)).To(ContainElement("very-secret"))
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			createSecret(fixture.Namespace, "top-secret", map[string]string{"worst-year-ever": "2016"})
		})

		It("updates the existing secret", func() {
			_, err := secretClient.Update(fixture.Namespace, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "top-secret",
					Labels: map[string]string{
						"worst-year-ever": "2020",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			secret, err := getSecret(fixture.Namespace, "top-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Labels).To(HaveKeyWithValue("worst-year-ever", "2020"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createSecret(fixture.Namespace, "open-secret", nil)
		})

		It("deletes a Secret", func() {
			Eventually(func() []string {
				return secretNames(listSecrets(fixture.Namespace))
			}).Should(ContainElement("open-secret"))

			err := secretClient.Delete(fixture.Namespace, "open-secret")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string {
				return secretNames(listSecrets(fixture.Namespace))
			}).ShouldNot(ContainElement("open-secret"))
		})
	})
})

var _ = Describe("Events", func() {
	var eventClient *client.Event

	BeforeEach(func() {
		eventClient = client.NewEvent(fixture.Clientset)
	})

	Describe("GetByPod", func() {
		var pod corev1.Pod

		BeforeEach(func() {
			pod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "the-pod",
					Namespace: fixture.Namespace,
					UID:       types.UID(tests.GenerateGUID()),
				},
			}

			createEvent(fixture.Namespace, "the-event", corev1.ObjectReference{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       pod.UID,
			})

			createEvent(fixture.Namespace, "another-event", corev1.ObjectReference{
				Name:      "another-pod",
				Namespace: fixture.Namespace,
				UID:       types.UID(tests.GenerateGUID()),
			})
		})

		It("lists the events beloging to a pod", func() {
			Eventually(func() []string {
				events, err := eventClient.GetByPod(pod)
				Expect(err).NotTo(HaveOccurred())

				return eventNames(events)
			}).Should(ConsistOf("the-event"))
		})
	})

	Describe("GetByInstanceAndReason", func() {
		var ownerRef corev1.ObjectReference

		BeforeEach(func() {
			ownerRef = corev1.ObjectReference{
				Name:      "the-owner",
				Namespace: fixture.Namespace,
				UID:       types.UID(tests.GenerateGUID()),
				Kind:      "the-kind",
			}

			createCrashEvent(fixture.Namespace, "the-crash-event", ownerRef, events.CrashEvent{
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Index:  42,
					Reason: "the-reason",
				},
			})

			createCrashEvent(fixture.Namespace, "another-crash-event", ownerRef, events.CrashEvent{
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Index:  43,
					Reason: "the-reason",
				},
			})

			createCrashEvent(fixture.Namespace, "yet-another-crash-event", ownerRef, events.CrashEvent{
				AppCrashedRequest: cc_messages.AppCrashedRequest{
					Index:  42,
					Reason: "another-reason",
				},
			})
		})

		It("lists the events beloging to an LRP instance with a specific reason", func() {
			Eventually(func() string {
				event, err := eventClient.GetByInstanceAndReason(fixture.Namespace, metav1.OwnerReference{
					Name: ownerRef.Name,
					UID:  ownerRef.UID,
					Kind: ownerRef.Kind,
				}, 42, "the-reason")
				Expect(err).NotTo(HaveOccurred())

				return event.Name
			}).Should(Equal("the-crash-event"))
		})
	})

	Describe("Create", func() {
		It("creates the secret in the namespace", func() {
			_, createErr := eventClient.Create(fixture.Namespace, &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a-party",
					Namespace: fixture.Namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Namespace: fixture.Namespace,
				},
			})
			Expect(createErr).NotTo(HaveOccurred())

			events := listEvents(fixture.Namespace)

			Expect(eventNames(events)).To(ContainElement("a-party"))
		})
	})

	Describe("Update", func() {
		var event *corev1.Event

		BeforeEach(func() {
			event = createEvent(fixture.Namespace, "whatever", corev1.ObjectReference{Namespace: fixture.Namespace})
		})

		It("updates the existing secret", func() {
			event.Reason = "the reason"
			_, err := eventClient.Update(fixture.Namespace, event)
			Expect(err).NotTo(HaveOccurred())

			event, err := getEvent(fixture.Namespace, "whatever")

			Expect(err).NotTo(HaveOccurred())
			Expect(event.Reason).To(Equal("the reason"))
		})
	})
})

func podNames(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}

	return names
}

func statefulSetNames(statefulSets []appsv1.StatefulSet) []string {
	names := make([]string, 0, len(statefulSets))
	for _, statefulSet := range statefulSets {
		names = append(names, statefulSet.Name)
	}

	return names
}

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

func secretNames(secrets []corev1.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, s := range secrets {
		names = append(names, s.Name)
	}

	return names
}

func eventNames(events []corev1.Event) []string {
	names := make([]string, 0, len(events))
	for _, e := range events {
		names = append(names, e.Name)
	}

	return names
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
