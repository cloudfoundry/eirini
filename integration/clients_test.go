package integration_test

import (
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Pod", func() {
	var podClient *client.Pod

	BeforeEach(func() {
		podClient = client.NewPod(fixture.Clientset)
	})

	Describe("GetAll", func() {
		var extraNs string

		BeforeEach(func() {
			extraNs = fixture.CreateExtraNamespace()

			createPods(fixture.Namespace, "one", "two", "three")
			createPods(extraNs, "four", "five", "six")
		})

		It("lists all pods across all namespaces", func() {
			Eventually(func() []string {
				pods, err := podClient.GetAll()
				Expect(err).NotTo(HaveOccurred())

				return podNames(pods)
			}).Should(ContainElements("one", "two", "three", "four", "five", "six"))
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			createPods(fixture.Namespace, "one", "two", "three")

			guid = util.GenerateGUID()

			createPod(fixture.Namespace, "four", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})
			createPod(fixture.Namespace, "five", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createPod(extraNs, "six", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
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
			createPods(fixture.Namespace, "foo")
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
		statefulSetClient = client.NewStatefulSet(fixture.Clientset)
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
			guid = util.GenerateGUID()

			createStatefulSet(fixture.Namespace, "foo", map[string]string{
				k8s.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "foo", nil)
		})

		It("retrieves a StatefulSet by namespace and name", func() {
			statefulSet, err := statefulSetClient.Get(fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(statefulSet.Name).To(Equal("foo"))
			Expect(statefulSet.Labels[k8s.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("GetBySourceType", func() {
		var extraNs string

		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "one", map[string]string{
				k8s.LabelSourceType: "FOO",
			})
			createStatefulSet(fixture.Namespace, "two", map[string]string{
				k8s.LabelSourceType: "BAR",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "three", map[string]string{
				k8s.LabelSourceType: "FOO",
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
			guid = util.GenerateGUID()

			createStatefulSet(fixture.Namespace, "one", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "two", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
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
		jobsClient = client.NewJob(fixture.Clientset, "my-eirini")
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
									Image:           "busybox",
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
				k8s.LabelGUID:           "bar",
				k8s.LabelEiriniInstance: "my-eirini",
			})
		})

		It("gets all jobs matching the specified guid and eirini instance id", func() {
			Eventually(func() []string {
				jobs, err := jobsClient.GetByGUID("bar")
				Expect(err).NotTo(HaveOccurred())

				return jobNames(jobs)
			}).Should(ContainElements("foo"))
		})
	})
})

var _ = Describe("StagingJobs", func() {
	var jobsClient *client.Job

	BeforeEach(func() {
		jobsClient = client.NewStagingJob(fixture.Clientset, "my-eirini")
	})

	Describe("GetByGUID", func() {
		BeforeEach(func() {
			createJob(fixture.Namespace, "foo", map[string]string{
				k8s.LabelStagingGUID:    "bar",
				k8s.LabelEiriniInstance: "my-eirini",
			})
		})

		It("gets all jobs matching the specified guid and eirini instance id", func() {
			Eventually(func() []string {
				jobs, err := jobsClient.GetByGUID("bar")
				Expect(err).NotTo(HaveOccurred())

				return jobNames(jobs)
			}).Should(ContainElements("foo"))
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
			guid = util.GenerateGUID()

			createSecret(fixture.Namespace, "foo", map[string]string{
				k8s.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createSecret(extraNs, "foo", nil)
		})

		It("retrieves a Secret by namespace and name", func() {
			secret, err := secretClient.Get(fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(secret.Name).To(Equal("foo"))
			Expect(secret.Labels[k8s.LabelGUID]).To(Equal(guid))
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

func secretNames(secrets []corev1.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, s := range secrets {
		names = append(names, s.Name)
	}

	return names
}
