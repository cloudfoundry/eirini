package k8s_test

import (
	"errors"
	"strconv"
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

		It("should provide the process-guid to the pod annotations", func() {
			statefulSet, _ := client.AppsV1beta2().StatefulSets(namespace).Get("Baldur", meta.GetOptions{})
			Expect(statefulSet.Spec.Template.Annotations[cf.ProcessGUID]).To(Equal("Baldur"))
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

			var (
				appName string
			)

			getStatefulSet := func(appName string) *v1beta2.StatefulSet {
				statefulSet, getErr := client.AppsV1beta2().StatefulSets(namespace).Get(appName, meta.GetOptions{})
				Expect(getErr).ToNot(HaveOccurred())
				return statefulSet
			}

			BeforeEach(func() {
				appName = "update"

				lrp := createLRP("update", "7653.2", `["my.example.route"]`)

				statefulSet := toStatefulSet(lrp)
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(statefulSet)
				Expect(createErr).NotTo(HaveOccurred())
			})

			Context("with replica count modified", func() {

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

			Context("with modified routes", func() {

				JustBeforeEach(func() {
					err = statefulSetDesirer.Update(&opi.LRP{
						Name:     appName,
						Metadata: map[string]string{cf.VcapAppUris: `["my.example.route", "my.second.example.route"]`}})
				})

				It("should update the stored routes", func() {
					Eventually(func() string {
						return getStatefulSet(appName).Annotations[eirini.RegisteredRoutes]
					}, 1*time.Second).Should(Equal(`["my.example.route", "my.second.example.route"]`))
				})
			})
		})

		Context("when the app does not exist", func() {

			var (
				appName string
			)

			JustBeforeEach(func() {
				err = statefulSetDesirer.Update(&opi.LRP{Name: appName, TargetInstances: 2})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not create the app", func() {
				_, err = client.AppsV1beta2().StatefulSets(namespace).Get(appName, meta.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When listing apps", func() {

		var (
			actualLRPs []*opi.LRP
		)

		BeforeEach(func() {
			for _, l := range lrps {
				_, createErr := client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l))
				Expect(createErr).ToNot(HaveOccurred())
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
				_, err = client.AppsV1beta2().StatefulSets(namespace).Create(toStatefulSet(l))
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("deletes the statefulSet", func() {
			err = statefulSetDesirer.Stop("odin")
			Expect(err).ToNot(HaveOccurred())

			Eventually(listStatefulSets, timeout).Should(HaveLen(2))
			Expect(getStatefulSetNames(listStatefulSets())).To(ConsistOf("mimir", "thor"))
		})

		Context("when the statefulSet does not exist", func() {

			JustBeforeEach(func() {
				err = statefulSetDesirer.Stop("test-app-where-are-you")
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Get LRP instances", func() {

		var (
			instances []*opi.Instance
			pod1      *v1.Pod
			pod2      *v1.Pod
		)

		BeforeEach(func() {
			since1 := meta.Unix(123, 0)
			pod1 = toPod("odin", 0, &since1)
			since2 := meta.Unix(456, 0)
			pod2 = toPod("odin", 1, &since2)
		})

		JustBeforeEach(func() {
			_, err = client.CoreV1().Pods(namespace).Create(pod1)
			Expect(err).ToNot(HaveOccurred())

			_, err = client.CoreV1().Pods(namespace).Create(pod2)
			Expect(err).ToNot(HaveOccurred())

			instances, err = statefulSetDesirer.GetInstances("odin")
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the correct number of instances", func() {
			Expect(instances).To(HaveLen(2))
			Expect(instances[0]).To(Equal(toInstance(0, 123000000000, "RUNNING")))
			Expect(instances[1]).To(Equal(toInstance(1, 456000000000, "RUNNING")))
		})

		Context("and time since creation is not available yet", func() {

			BeforeEach(func() {
				pod1 = toPod("mimir", 0, nil)
				since2 := meta.Unix(456, 0)
				pod2 = toPod("mimir", 1, &since2)
			})

			JustBeforeEach(func() {
				instances, err = statefulSetDesirer.GetInstances("mimir")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a default value", func() {
				Expect(instances).To(HaveLen(2))
				Expect(instances[0]).To(Equal(toInstance(0, 0, "RUNNING")))
				Expect(instances[1]).To(Equal(toInstance(1, 456000000000, "RUNNING")))
			})
		})

		Context("and the pod has crashed", func() {
			BeforeEach(func() {
				pod1.Status.ContainerStatuses[0].State = v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{},
				}

				pod2.Status.ContainerStatuses[0].State = v1.ContainerState{
					Waiting: &v1.ContainerStateWaiting{},
				}
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a default value", func() {
				Expect(instances).To(HaveLen(2))
				Expect(instances[0]).To(Equal(toInstance(0, 123000000000, "CRASHED")))
				Expect(instances[1]).To(Equal(toInstance(1, 456000000000, "CRASHED")))
			})
		})

		Context("and the pod is pending", func() {
			BeforeEach(func() {
				pod1.Status.Phase = v1.PodPending
				pod2.Status.ContainerStatuses[0].Ready = false
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a default value", func() {
				Expect(instances).To(HaveLen(2))
				Expect(instances[0]).To(Equal(toInstance(0, 123000000000, "CLAIMED")))
				Expect(instances[1]).To(Equal(toInstance(1, 456000000000, "CLAIMED")))
			})
		})

		Context("and the pod phase is unknown", func() {
			BeforeEach(func() {
				pod1.Status.Phase = v1.PodUnknown
				pod2.Status.Phase = v1.PodUnknown
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a default value", func() {
				Expect(instances).To(HaveLen(2))
				Expect(instances[0]).To(Equal(toInstance(0, 123000000000, "UNKNOWN")))
				Expect(instances[1]).To(Equal(toInstance(1, 456000000000, "UNKNOWN")))
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

func toPod(lrpName string, index int, time *meta.Time) *v1.Pod {
	pod := v1.Pod{}
	pod.Name = lrpName + "-" + strconv.Itoa(index)
	pod.Labels = map[string]string{
		"name": lrpName,
	}

	pod.Status.StartTime = time
	pod.Status.Phase = v1.PodRunning
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		{
			State: v1.ContainerState{Running: &v1.ContainerStateRunning{}},
			Ready: true,
		},
	}
	return &pod
}

func toInstance(index int, since int64, state string) *opi.Instance {
	return &opi.Instance{
		Index: index,
		Since: since,
		State: state,
	}
}

func toStatefulSet(lrp *opi.LRP) *v1beta2.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	fieldEnvs := []v1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "CF_INSTANCE_IP",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: "CF_INSTANCE_INTERNAL_IP",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
	}

	envs = append(envs, fieldEnvs...)

	targetInstances := int32(lrp.TargetInstances)
	statefulSet := &v1beta2.StatefulSet{
		Spec: v1beta2.StatefulSetSpec{
			Replicas: &targetInstances,
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Annotations: map[string]string{
						cf.ProcessGUID: lrp.Metadata[cf.ProcessGUID],
					},
				},
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
