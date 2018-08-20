package k8s_test

import (
	"errors"
	"time"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
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
		serviceManager     *k8sfakes.FakeServiceManager
		probeCreator       *k8sfakes.FakeLivenessProbeCreator
		lrps               []*opi.LRP
	)

	const (
		namespace               = "testing"
		timeout   time.Duration = 60 * time.Second
	)

	listStatefulSets := func() []v1beta2.StatefulSet {
		list, listErr := client.AppsV1beta2().StatefulSets(namespace).List(meta.ListOptions{})
		Expect(listErr).NotTo(HaveOccurred())
		return list.Items
	}

	BeforeEach(func() {
		lrps = []*opi.LRP{
			createLRP("odin", "1234.5"),
			createLRP("thor", "4567.8"),
			createLRP("mimir", "9012.3"),
		}

		client = fake.NewSimpleClientset()
		serviceManager = new(k8sfakes.FakeServiceManager)
		probeCreator = new(k8sfakes.FakeLivenessProbeCreator)
	})

	JustBeforeEach(func() {
		statefulSetManager = &StatefulSetManager{
			Client:               client,
			Namespace:            namespace,
			ServiceManager:       serviceManager,
			LivenessProbeCreator: probeCreator.Spy,
		}
	})

	Context("When creating an LRP", func() {
		var lrp *opi.LRP

		JustBeforeEach(func() {
			probeCreator.Returns(&v1.Probe{})
			lrp = createLRP("Baldur", "1234.5")
			err = statefulSetManager.Create(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the desired statefulSet", func() {
			statefulSet, getErr := client.AppsV1beta2().StatefulSets(namespace).Get("Baldur", meta.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			Expect(statefulSet).To(Equal(toStatefulSet(lrp, namespace)))
		})

		It("creates a healthcheck probe", func() {
			Expect(probeCreator.CallCount()).To(Equal(1))
		})

		It("should create a headless service", func() {
			Expect(serviceManager.CreateHeadlessCallCount()).To(Equal(1))
			headlessLRP := serviceManager.CreateHeadlessArgsForCall(0)
			Expect(headlessLRP).To(Equal(lrp))
		})

		Context("When redeploying an existing LRP", func() {
			BeforeEach(func() {
				lrp = createLRP("Baldur", "1234.5")
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(lrp, namespace))
				Expect(createErr).ToNot(HaveOccurred())
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not create headless service", func() {
				Expect(serviceManager.CreateHeadlessCallCount()).To(Equal(0))
			})
		})

		Context("When fails to create a headless service", func() {

			BeforeEach(func() {
				serviceManager.CreateHeadlessReturns(errors.New("oopsie"))
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When getting an app", func() {

		var lrp *opi.LRP

		BeforeEach(func() {
			for _, l := range lrps {
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l, namespace))
				Expect(createErr).ToNot(HaveOccurred())
			}
		})

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
				appName = "baldur"
				lrp := createLRP(appName, "9012.3")
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(lrp, namespace))
				Expect(createErr).ToNot(HaveOccurred())
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
			Context("with replica count modified", func() {

				var (
					err     error
					appName string
				)

				getStatefulSet := func(appName string) *v1beta2.StatefulSet {
					statefulSet, getErr := client.AppsV1beta2().StatefulSets(namespace).Get(appName, meta.GetOptions{})
					Expect(getErr).ToNot(HaveOccurred())
					return statefulSet
				}

				BeforeEach(func() {
					appName = "update"

					lrp := createLRP("update", "7653.2")

					statefulSet := toStatefulSet(lrp, namespace)
					_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(statefulSet)
					Expect(createErr).NotTo(HaveOccurred())
				})

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

		BeforeEach(func() {
			for _, l := range lrps {
				_, err := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l, namespace))
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("translates all existing statefulSets to opi.LRPs", func() {
			actualLRPs, err := statefulSetManager.List()
			Expect(err).ToNot(HaveOccurred())
			Expect(actualLRPs).To(ConsistOf(lrps))
		})

		Context("When no statefulSets exist", func() {

			BeforeEach(func() {
				client = fake.NewSimpleClientset()
				serviceManager = new(k8sfakes.FakeServiceManager)
			})

			It("returns an empy list of LRPs", func() {
				actualLRPs, err := statefulSetManager.List()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualLRPs).To(BeEmpty())
			})
		})
	})

	Context("Delete a statefulSet", func() {

		BeforeEach(func() {
			for _, l := range lrps {
				_, err := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l, namespace))
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("deletes the statefulSet", func() {
			err := statefulSetManager.Delete("odin")
			Expect(err).ToNot(HaveOccurred())

			Eventually(listStatefulSets, timeout).Should(HaveLen(2))
			Expect(getStatefulSetNames(listStatefulSets())).To(ConsistOf("mimir", "thor"))
		})

		It("deletes the associated headless service", func() {
			err := statefulSetManager.Delete("odin")
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceManager.DeleteHeadlessCallCount()).To(Equal(1))
			appName := serviceManager.DeleteHeadlessArgsForCall(0)
			Expect(appName).To(Equal("odin"))
		})

		Context("when the statefulSet does not exist", func() {

			var err error

			JustBeforeEach(func() {
				err = statefulSetManager.Delete("test-app-where-are-you")
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("does not delete a headless service", func() {
				Expect(serviceManager.DeleteHeadlessCallCount()).To(Equal(0))
			})

		})

		Context("when the headless service cannot be deleted", func() {

			BeforeEach(func() {
				serviceManager.DeleteHeadlessReturns(errors.New("oopsie"))
			})

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
		Name: "POD_NAME",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.name",
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
						{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     envs,
							Ports: []v1.ContainerPort{
								{
									Name:          "expose",
									ContainerPort: 8080,
								},
							},
							LivenessProbe: &v1.Probe{},
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
		RunningInstances: 0,
		Image:            "busybox",
		Metadata: map[string]string{
			cf.ProcessGUID: processGUID,
			cf.LastUpdated: lastUpdated,
		},
	}
}
