package k8s_test

import (
	"fmt"
	"time"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var _ = Describe("Statefulset", func() {
	var (
		err                error
		client             kubernetes.Interface
		statefulSetManager InstanceManager
		lrps               []*opi.LRP
	)

	const (
		namespace               = "testing"
		timeout   time.Duration = 60 * time.Second
	)

	namespaceExists := func(name string) bool {
		_, err := client.CoreV1().Namespaces().Get(namespace, meta.GetOptions{})
		return err == nil
	}

	createNamespace := func(name string) {
		namespaceSpec := &v1.Namespace{
			ObjectMeta: meta.ObjectMeta{Name: name},
		}

		if _, err := client.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
			panic(err)
		}
	}

	getLRPNames := func() []string {
		names := []string{}
		for _, lrp := range lrps {
			names = append(names, lrp.Metadata[cf.ProcessGUID])
		}
		return names
	}

	listStatefulSets := func() []v1beta2.StatefulSet {
		list, err := client.AppsV1beta2().StatefulSets(namespace).List(meta.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		return list.Items
	}

	listPods := func(appName string) []v1.Pod {
		labelSelector := fmt.Sprintf("name=%s", appName)
		pods, err := client.CoreV1().Pods(namespace).List(meta.ListOptions{LabelSelector: labelSelector})
		Expect(err).NotTo(HaveOccurred())
		return pods.Items
	}

	listReplicasets := func(appName string) []v1beta2.ReplicaSet {
		labelSelector := fmt.Sprintf("name=%s", appName)
		replicasets, err := client.AppsV1beta2().ReplicaSets(namespace).List(meta.ListOptions{LabelSelector: labelSelector})
		Expect(err).NotTo(HaveOccurred())
		return replicasets.Items
	}

	cleanupStatefulSet := func(appName string) {
		backgroundPropagation := meta.DeletePropagationBackground
		client.AppsV1beta2().StatefulSets(namespace).Delete(appName, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation})
	}

	BeforeEach(func() {
		lrps = []*opi.LRP{
			createLRP("odin", "1234.5"),
			createLRP("thor", "4567.8"),
			createLRP("mimir", "9012.3"),
		}

	})

	AfterEach(func() {
		for _, appName := range getLRPNames() {
			cleanupStatefulSet(appName)
		}

		Eventually(listStatefulSets, timeout).Should(BeEmpty())
	})

	JustBeforeEach(func() {
		client = fake.NewSimpleClientset()
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		for _, l := range lrps {
			client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l, namespace))
		}

		statefulSetManager = NewStatefulSetManager(client, namespace)
	})

	Context("When creating an LRP", func() {
		var lrp *opi.LRP

		JustBeforeEach(func() {
			lrp = createLRP("Baldur", "1234.5")
			lrps = append(lrps, lrp)

			err = statefulSetManager.Create(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the desired statefulSet", func() {
			statefulSet, err := client.AppsV1beta2().StatefulSets(namespace).Get("Baldur", meta.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(statefulSet).To(Equal(toStatefulSet(lrp, namespace)))
		})

		Context("When redeploying an existing LRP", func() {
			BeforeEach(func() {
				lrp = createLRP("Baldur", "1234.5")
				lrps = append(lrps, lrp)
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When getting an app", func() {

		var lrp *opi.LRP

		JustBeforeEach(func() {
			lrp, err = statefulSetManager.Get("odin")
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("return the expected LRP", func() {
			Expect(lrps).To(ContainElement(lrp))
		})

		Context("when the app does not exist", func() {
			JustBeforeEach(func() {
				lrp, err = statefulSetManager.Get("non-existent")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Context("When checking if an app exists", func() {

		var (
			exists  bool
			appName string
		)

		JustBeforeEach(func() {
			exists, err = statefulSetManager.Exists(appName)
		})

		Context("when the app exists", func() {

			BeforeEach(func() {
				appName = "mimir"
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("shold return true", func() {
				Expect(exists).To(Equal(true))
			})
		})

		Context("when the app does not exist", func() {

			BeforeEach(func() {
				appName = "non-existent"
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("shold return true", func() {
				Expect(exists).To(Equal(false))
			})
		})
	})

	Context("When updating an app", func() {
		Context("when the app exists", func() {

			var (
				appName string
			)

			BeforeEach(func() {
				appName = "update"

				lrp := createLRP("update", "7653.2")
				lrps = append(lrps, lrp)

				statefulSet := toStatefulSet(lrp, namespace)
				_, err := client.AppsV1beta2().StatefulSets(namespace).Create(statefulSet)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with replica count modified", func() {

				var err error

				getStatefulSet := func(appName string) *v1beta2.StatefulSet {
					statefulSet, err := client.AppsV1beta2().StatefulSets(namespace).Get(appName, meta.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return statefulSet
				}

				JustBeforeEach(func() {
					err = statefulSetManager.Update(&opi.LRP{
						Name:            appName,
						TargetInstances: 5,
						Metadata:        map[string]string{cf.LastUpdated: "123214.2"}})
				})

				It("scales the app without error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("updates the desired number of app instances", func() {
					Eventually(func() int32 {
						return *getStatefulSet(appName).Spec.Replicas
					}, timeout).Should(Equal(int32(5)))
				})

				PIt("creates the desired number of app instances", func() {
					Eventually(func() int32 {
						return getStatefulSet(appName).Status.CurrentReplicas
					}, timeout).Should(Equal(int32(5)))

					Eventually(func() int32 {
						return getStatefulSet(appName).Status.UpdatedReplicas
					}, timeout).Should(Equal(int32(5)))
				})

				PIt("updates the timestamp of the last update", func() {
					Eventually(func() map[string]string {
						return getStatefulSet(appName).Annotations
					}, timeout).Should(HaveKeyWithValue(cf.LastUpdated, "123214.2"))
				})
			})
		})

		Context("when the app does not exist", func() {

			var (
				err     error
				appName string
			)

			JustBeforeEach(func() {
				err = statefulSetManager.Update(&opi.LRP{Name: appName, TargetInstances: 2})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not create the app", func() {
				_, err := client.AppsV1beta2().StatefulSets(namespace).Get(appName, meta.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("List StatefulSets", func() {
		It("translates all existing statefulSets to opi.LRPs", func() {
			actualLRPs, err := statefulSetManager.List()
			Expect(err).ToNot(HaveOccurred())
			Expect(actualLRPs).To(ConsistOf(lrps))
		})

		Context("When no statefulSets exist", func() {

			BeforeEach(func() {
				lrps = []*opi.LRP{}
			})

			It("returns an empy list of LRPs", func() {
				actualLRPs, err := statefulSetManager.List()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualLRPs).To(BeEmpty())
			})
		})
	})

	Context("Delete a statefulSet", func() {

		It("deletes the statefulSet", func() {
			err := statefulSetManager.Delete("odin")
			Expect(err).ToNot(HaveOccurred())

			Eventually(listStatefulSets, timeout).Should(HaveLen(2))
			Expect(getStatefulSetNames(listStatefulSets())).To(ConsistOf("mimir", "thor"))
		})

		PIt("deletes the pods associated with the statefulSet", func() {
			err := statefulSetManager.Delete("odin")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []v1.Pod {
				return listPods("odin")
			}, timeout).Should(BeEmpty())
		})

		It("deletes the replicasets associated with the statefulSet", func() {
			err := statefulSetManager.Delete("odin")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []v1beta2.ReplicaSet {
				return listReplicasets("odin")
			}, timeout).Should(BeEmpty())
		})

		Context("when the statefulSet does not exist", func() {

			It("returns an error", func() {
				err := statefulSetManager.Delete("test-app-where-are-you")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func getStatefulSetNames(statefulSets []v1beta2.StatefulSet) []string {
	statefulSetNames := []string{}
	for _, d := range statefulSets {
		statefulSetNames = append(statefulSetNames, d.Name)
	}
	return statefulSetNames
}

func toStatefulSet(lrp *opi.LRP, namespace string) *v1beta2.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	envs = append(envs, v1.EnvVar{
		Name: "INSTANCE_INDEX",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.annotations['spec.pod.beta.kubernetes.io/statefulset-index']",
			},
		},
	})

	targetInstances := int32(lrp.TargetInstances)
	statefulSet := &v1beta2.StatefulSet{
		Spec: v1beta2.StatefulSetSpec{
			Replicas: &targetInstances,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     envs,
							Ports: []v1.ContainerPort{
								v1.ContainerPort{
									Name:          "expose",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	statefulSet.Name = lrp.Name

	statefulSet.Namespace = namespace
	statefulSet.Spec.Template.Labels = map[string]string{
		"name": lrp.Name,
	}

	statefulSet.Spec.Selector = &meta.LabelSelector{
		MatchLabels: map[string]string{
			"name": lrp.Name,
		},
	}

	statefulSet.Labels = map[string]string{
		"eirini": "eirini",
		"name":   lrp.Name,
	}

	statefulSet.Annotations = lrp.Metadata

	return statefulSet
}

func createLRP(processGUID, lastUpdated string) *opi.LRP {
	return &opi.LRP{
		Name: processGUID,
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		Image: "busybox",
		Metadata: map[string]string{
			cf.ProcessGUID: processGUID,
			cf.LastUpdated: lastUpdated,
		},
	}
}
