package k8s_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	testcore "k8s.io/client-go/testing"
)

const (
	namespace               = "testing"
	timeout   time.Duration = 60 * time.Second
)

var _ = Describe("Statefulset", func() {

	var (
		err                   error
		client                kubernetes.Interface
		statefulSetDesirer    opi.Desirer
		livenessProbeCreator  *k8sfakes.FakeProbeCreator
		readinessProbeCreator *k8sfakes.FakeProbeCreator
		lrps                  []*opi.LRP
	)

	listStatefulSets := func() []v1beta2.StatefulSet {
		list, listErr := client.AppsV1beta2().StatefulSets(namespace).List(meta.ListOptions{})
		Expect(listErr).NotTo(HaveOccurred())
		return list.Items
	}

	BeforeEach(func() {
		lrps = []*opi.LRP{
			createLRP("odin", "1234.5", "my.example.route"),
			createLRP("thor", "4567.8", "my.example.route"),
			createLRP("mimir", "9012.3", "my.example.route"),
		}

		client = fake.NewSimpleClientset()
		livenessProbeCreator = new(k8sfakes.FakeProbeCreator)
		readinessProbeCreator = new(k8sfakes.FakeProbeCreator)
	})

	JustBeforeEach(func() {
		statefulSetDesirer = &StatefulSetDesirer{
			Client:                client,
			Namespace:             namespace,
			LivenessProbeCreator:  livenessProbeCreator.Spy,
			ReadinessProbeCreator: readinessProbeCreator.Spy,
		}
	})

	Context("When creating an LRP", func() {
		var lrp *opi.LRP

		JustBeforeEach(func() {
			livenessProbeCreator.Returns(&v1.Probe{})
			readinessProbeCreator.Returns(&v1.Probe{})
			lrp = createLRP("Baldur", "1234.5", "my.example.route")
			err = statefulSetDesirer.Desire(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the desired statefulSet", func() {
			statefulSet, getErr := client.AppsV1beta2().StatefulSets(namespace).Get("Baldur", meta.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			Expect(statefulSet).To(Equal(toStatefulSet(lrp)))
		})

		It("should creates a healthcheck probe", func() {
			Expect(livenessProbeCreator.CallCount()).To(Equal(1))
		})

		It("should creates a readiness probe", func() {
			Expect(readinessProbeCreator.CallCount()).To(Equal(1))
		})

		Context("When redeploying an existing LRP", func() {
			BeforeEach(func() {
				lrp = createLRP("Baldur", "1234.5", "my.example.route")
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(lrp))
				Expect(createErr).ToNot(HaveOccurred())
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
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l))
				Expect(createErr).ToNot(HaveOccurred())
			}
		})

		JustBeforeEach(func() {
			lrp, err = statefulSetDesirer.Get("odin")
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("return the expected LRP", func() {
			Expect(lrps).To(ContainElement(lrp))
		})

		Context("when the app does not exist", func() {
			JustBeforeEach(func() {
				lrp, err = statefulSetDesirer.Get("non-existent")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
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

					lrp := createLRP("update", "7653.2", "my.example.route")

					statefulSet := toStatefulSet(lrp)
					_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(statefulSet)
					Expect(createErr).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					err = statefulSetDesirer.Update(&opi.LRP{
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
				err = statefulSetDesirer.Update(&opi.LRP{Name: appName, TargetInstances: 2})
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

	Context("When listing apps", func() {

		var (
			actualLRPs []*opi.LRP
			err        error
		)

		BeforeEach(func() {
			for _, l := range lrps {
				_, err := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l))
				Expect(err).ToNot(HaveOccurred())
			}
		})

		JustBeforeEach(func() {
			actualLRPs, err = statefulSetDesirer.List()
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("translates all existing statefulSets to opi.LRPs", func() {
			Expect(actualLRPs).To(ConsistOf(lrps))
		})

		Context("no statefulSets exist", func() {

			BeforeEach(func() {
				client = fake.NewSimpleClientset()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an empy list of LRPs", func() {
				Expect(actualLRPs).To(BeEmpty())
			})
		})

		Context("fails to list the statefulsets", func() {

			BeforeEach(func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.(*fake.Clientset).PrependReactor("list", "statefulsets", reaction)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Context("Stop an LRP", func() {

		BeforeEach(func() {
			for _, l := range lrps {
				_, err := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l))
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("deletes the statefulSet", func() {
			err := statefulSetDesirer.Stop("odin")
			Expect(err).ToNot(HaveOccurred())

			Eventually(listStatefulSets, timeout).Should(HaveLen(2))
			Expect(getStatefulSetNames(listStatefulSets())).To(ConsistOf("mimir", "thor"))
		})

		Context("when the statefulSet does not exist", func() {

			var err error

			JustBeforeEach(func() {
				err = statefulSetDesirer.Stop("test-app-where-are-you")
			})

			It("returns an error", func() {
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

func toStatefulSet(lrp *opi.LRP) *v1beta2.StatefulSet {
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
							LivenessProbe:  &v1.Probe{},
							ReadinessProbe: &v1.Probe{},
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
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]

	return statefulSet
}

func createLRP(processGUID, lastUpdated, routes string) *opi.LRP {
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
			cf.VcapAppUris: routes,
		},
	}
}
