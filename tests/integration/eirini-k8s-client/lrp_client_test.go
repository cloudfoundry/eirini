package integration_test

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/pdb"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("LRPClient", func() {
	var (
		allowRunImageAsRoot bool
		lrpClient           *k8s.LRPClient
		lrp                 *api.LRP
	)

	BeforeEach(func() {
		allowRunImageAsRoot = false
		lrp = createLRP("ödin")
	})

	AfterEach(func() {
		cleanupStatefulSet(lrp)
		Eventually(func() []appsv1.StatefulSet {
			return listStatefulSets(lrp)
		}).Should(BeEmpty())
	})

	JustBeforeEach(func() {
		lrpClient = createLrpClient(fixture.Namespace, allowRunImageAsRoot)
	})

	Describe("Desire", func() {
		var desireErr error

		JustBeforeEach(func() {
			desireErr = lrpClient.Desire(ctx, fixture.Namespace, lrp)
		})

		It("succeeds", func() {
			Expect(desireErr).NotTo(HaveOccurred())
		})

		// join all tests in a single with By()
		It("should create a StatefulSet object", func() {
			statefulset := getStatefulSetForLRP(lrp)
			Expect(statefulset.Name).To(ContainSubstring(lrp.GUID))
			Expect(statefulset.Namespace).To(Equal(fixture.Namespace))
			Expect(statefulset.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "registry-secret"}))
			Expect(statefulset.Annotations[stset.AnnotationOriginalRequest]).To(Equal(lrp.LRP))
			Expect(statefulset.Labels).To(SatisfyAll(
				HaveKeyWithValue(stset.LabelGUID, lrp.GUID),
				HaveKeyWithValue(stset.LabelVersion, lrp.Version),
				HaveKeyWithValue(stset.LabelSourceType, "APP"),
				HaveKeyWithValue(stset.LabelAppGUID, "the-app-guid"),
			))

			Expect(statefulset.Spec.Replicas).To(Equal(int32ptr(lrp.TargetInstances)))
			Expect(statefulset.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(BeTrue()))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Command).To(Equal(lrp.Command))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(Equal(lrp.Image))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
		})

		It("sets the latest migration index annotation", func() {
			statefulset := getStatefulSetForLRP(lrp)
			Expect(statefulset.Annotations).To(HaveKeyWithValue(shared.AnnotationLatestMigration, strconv.Itoa(123)))
		})

		It("should create all associated pods", func() {
			var podNames []string

			Eventually(func() []string {
				podNames = podNamesFromPods(listPods(lrp.LRPIdentifier))

				return podNames
			}).Should(HaveLen(lrp.TargetInstances))

			for i := 0; i < lrp.TargetInstances; i++ {
				podIndex := i
				Expect(podNames[podIndex]).To(ContainSubstring(lrp.GUID))

				Eventually(func() string {
					return getPodPhase(podIndex, lrp.LRPIdentifier)
				}).Should(Equal("Ready"))
			}

			Eventually(func() int32 {
				return getStatefulSetForLRP(lrp).Status.ReadyReplicas
			}).Should(Equal(int32(2)))
		})

		It("should create a pod disruption budget for the lrp", func() {
			statefulset := getStatefulSetForLRP(lrp)
			pdb, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pdb).NotTo(BeNil())
			Expect(pdb.Spec.MinAvailable).To(PointTo(Equal(intstr.FromString("50%"))))
			Expect(pdb.Spec.MaxUnavailable).To(BeNil())
		})

		When("the lrp has 1 instance", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 1
			})

			It("should not create a pod disruption budget for the lrp", func() {
				_, err := podDisruptionBudgets().Get(context.Background(), "ödin", metav1.GetOptions{})
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})

		When("additional app info is provided", func() {
			BeforeEach(func() {
				lrp.OrgName = "odin-org"
				lrp.OrgGUID = "odin-org-guid"
				lrp.SpaceName = "odin-space"
				lrp.SpaceGUID = "odin-space-guid"
			})

			DescribeTable("sets appropriate annotations to statefulset", func(key, value string) {
				statefulset := getStatefulSetForLRP(lrp)
				Expect(statefulset.Annotations).To(HaveKeyWithValue(key, value))
			},
				Entry("SpaceName", stset.AnnotationSpaceName, "odin-space"),
				Entry("SpaceGUID", stset.AnnotationSpaceGUID, "odin-space-guid"),
				Entry("OrgName", stset.AnnotationOrgName, "odin-org"),
				Entry("OrgGUID", stset.AnnotationOrgGUID, "odin-org-guid"),
			)

			It("sets appropriate labels to statefulset", func() {
				statefulset := getStatefulSetForLRP(lrp)
				Expect(statefulset.Labels).To(HaveKeyWithValue(stset.LabelGUID, lrp.LRPIdentifier.GUID))
				Expect(statefulset.Labels).To(HaveKeyWithValue(stset.LabelVersion, lrp.LRPIdentifier.Version))
				Expect(statefulset.Labels).To(HaveKeyWithValue(stset.LabelSourceType, "APP"))
			})
		})

		When("the app has more than one instances", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 2
			})

			It("should schedule app pods on different nodes", func() {
				if getNodeCount() == 1 {
					Skip("target cluster has only one node")
				}

				Eventually(func() []corev1.Pod {
					return listPods(lrp.LRPIdentifier)
				}).Should(HaveLen(2))

				var nodeNames []string
				Eventually(func() []string {
					nodeNames = nodeNamesFromPods(listPods(lrp.LRPIdentifier))

					return nodeNames
				}).Should(HaveLen(2))
				Expect(nodeNames[0]).ToNot(Equal(nodeNames[1]))
			})
		})

		When("private docker registry credentials are provided", func() {
			BeforeEach(func() {
				lrp.Image = "eiriniuser/notdora:latest"
				lrp.Command = nil
				lrp.PrivateRegistry = &api.PrivateRegistry{
					Server:   "index.docker.io/v1/",
					Username: "eiriniuser",
					Password: tests.GetEiriniDockerHubPassword(),
				}
			})

			It("creates a private registry secret", func() {
				statefulset := getStatefulSetForLRP(lrp)
				Expect(statefulset.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(2))
				privateRegistrySecretName := statefulset.Spec.Template.Spec.ImagePullSecrets[1].Name
				secret, err := getSecret(fixture.Namespace, privateRegistrySecretName)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret).NotTo(BeNil())
			})

			It("sets the ImagePullSecret correctly in the pod template", func() {
				Eventually(func() []corev1.Pod {
					return listPods(lrp.LRPIdentifier)
				}).Should(HaveLen(lrp.TargetInstances))

				for i := 0; i < lrp.TargetInstances; i++ {
					podIndex := i
					Eventually(func() string {
						return getPodPhase(podIndex, lrp.LRPIdentifier)
					}).Should(Equal("Ready"))
				}
			})
		})

		When("we create the same StatefulSet again", func() {
			It("should not error", func() {
				err := lrpClient.Desire(ctx, fixture.Namespace, lrp)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("using a docker image that needs root access", func() {
			BeforeEach(func() {
				allowRunImageAsRoot = true

				lrp.Image = "eirini/nginx-integration"
				lrp.Command = nil
				lrp.Health.Type = "http"
				lrp.Health.Port = 8080
			})

			It("should start all the pods", func() {
				var podNames []string

				Eventually(func() []string {
					podNames = podNamesFromPods(listPods(lrp.LRPIdentifier))

					return podNames
				}).Should(HaveLen(lrp.TargetInstances))

				for i := 0; i < lrp.TargetInstances; i++ {
					podIndex := i
					Eventually(func() string {
						return getPodPhase(podIndex, lrp.LRPIdentifier)
					}).Should(Equal("Ready"))
				}

				Eventually(func() int32 {
					return getStatefulSetForLRP(lrp).Status.ReadyReplicas
				}).Should(BeNumerically("==", lrp.TargetInstances))
			})
		})

		When("the LRP has 0 target instances", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 0
			})

			It("still creates a statefulset, with 0 replicas", func() {
				statefulset := getStatefulSetForLRP(lrp)
				Expect(statefulset.Name).To(ContainSubstring(lrp.GUID))
				Expect(statefulset.Spec.Replicas).To(Equal(int32ptr(0)))
			})
		})

		When("the the app has sidecars", func() {
			assertEqualValues := func(actual, expected *resource.Quantity) {
				Expect(actual.Value()).To(Equal(expected.Value()))
			}

			BeforeEach(func() {
				lrp.Image = "eirini/busybox"
				lrp.Command = []string{"/bin/sh", "-c", "echo Hello from app; sleep 3600"}
				lrp.Sidecars = []api.Sidecar{
					{
						Name:     "the-sidecar",
						Command:  []string{"/bin/sh", "-c", "echo Hello from sidecar; sleep 3600"},
						MemoryMB: 101,
					},
				}
			})

			It("deploys the app with the sidcar container", func() {
				statefulset := getStatefulSetForLRP(lrp)
				Expect(statefulset.Spec.Template.Spec.Containers).To(HaveLen(2))
			})

			It("sets resource limits on the sidecar container", func() {
				statefulset := getStatefulSetForLRP(lrp)
				containers := statefulset.Spec.Template.Spec.Containers
				for _, container := range containers {
					if container.Name == "the-sidecar" {
						limits := container.Resources.Limits
						requests := container.Resources.Requests

						expectedMemory := resource.NewScaledQuantity(101, resource.Mega)
						expectedDisk := resource.NewScaledQuantity(lrp.DiskMB, resource.Mega)
						expectedCPU := resource.NewScaledQuantity(int64(lrp.CPUWeight*10), resource.Milli)

						assertEqualValues(limits.Memory(), expectedMemory)
						assertEqualValues(limits.StorageEphemeral(), expectedDisk)
						assertEqualValues(requests.Memory(), expectedMemory)
						assertEqualValues(requests.Cpu(), expectedCPU)
					}
				}
			})
		})

		When("the app has user defined annotations", func() {
			BeforeEach(func() {
				lrp.UserDefinedAnnotations = map[string]string{
					"prometheus.io/scrape": "yes, please",
				}
			})

			It("sets them on the pod template", func() {
				statefulset := getStatefulSetForLRP(lrp)
				Expect(statefulset.Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "yes, please"))
			})
		})
	})

	Describe("Stop", func() {
		var (
			statefulsetName  string
			imagePullSecrets []corev1.LocalObjectReference
		)

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, lrp)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []corev1.Pod {
				return listPods(lrp.LRPIdentifier)
			}).Should(HaveLen(lrp.TargetInstances))

			stSet := getStatefulSetForLRP(lrp)
			statefulsetName = stSet.Name
			imagePullSecrets = stSet.Spec.Template.Spec.ImagePullSecrets

			err = lrpClient.Stop(ctx, lrp.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the StatefulSet object", func() {
			Eventually(func() []appsv1.StatefulSet {
				return listStatefulSetsForApp("odin")
			}).Should(BeEmpty())
		})

		It("should delete the associated pods", func() {
			Eventually(func() []corev1.Pod {
				return listPods(lrp.LRPIdentifier)
			}).Should(BeEmpty())
		})

		It("should delete the pod disruption budget for the lrp", func() {
			Eventually(func() error {
				_, err := podDisruptionBudgets().Get(context.Background(), statefulsetName, metav1.GetOptions{})

				return err
			}).Should(MatchError(ContainSubstring("not found")))
		})

		When("the lrp has only 1 instance", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 1
			})

			It("keep the lrp without a pod disruption budget", func() {
				Eventually(func() error {
					_, err := podDisruptionBudgets().Get(context.Background(), statefulsetName, metav1.GetOptions{})

					return err
				}).Should(MatchError(ContainSubstring("not found")))
			})
		})

		When("private docker registry credentials are provided", func() {
			BeforeEach(func() {
				lrp.Image = "eiriniuser/notdora:latest"
				lrp.PrivateRegistry = &api.PrivateRegistry{
					Server:   "index.docker.io/v1/",
					Username: "eiriniuser",
					Password: tests.GetEiriniDockerHubPassword(),
				}
			})

			It("should delete the StatefulSet object", func() {
				Eventually(func() []appsv1.StatefulSet {
					return listStatefulSetsForApp("odin")
				}).Should(BeEmpty())
			})

			It("should delete the private registry secret", func() {
				Expect(imagePullSecrets).To(HaveLen(2))
				privateRegistrySecretName := imagePullSecrets[1].Name

				Eventually(func() error {
					_, err := getSecret(fixture.Namespace, privateRegistrySecretName)

					return err
				}).Should(MatchError(ContainSubstring("not found")))
			})
		})
	})

	Describe("Update", func() {
		Describe("scaling", func() {
			var (
				instancesBefore int
				instancesAfter  int
				statefulset     *appsv1.StatefulSet
			)

			BeforeEach(func() {
				instancesBefore = 1
				instancesAfter = 2
			})

			JustBeforeEach(func() {
				lrp.TargetInstances = instancesBefore
				Expect(lrpClient.Desire(ctx, fixture.Namespace, lrp)).To(Succeed())

				lrp.TargetInstances = instancesAfter
				Expect(lrpClient.Update(ctx, lrp)).To(Succeed())
				statefulset = getStatefulSetForLRP(lrp)
			})

			It("updates instance count", func() {
				Expect(statefulset.Spec.Replicas).To(PointTo(Equal(int32(2))))
			})

			When("scaling up from 1 to 2 instances", func() {
				It("should create a pod disruption budget for the lrp", func() {
					pdb, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pdb).NotTo(BeNil())
				})
			})

			When("scaling up from 2 to 3 instances", func() {
				BeforeEach(func() {
					instancesBefore = 2
					instancesAfter = 3
				})

				It("should keep the existing pod disruption budget for the lrp", func() {
					pdb, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pdb).NotTo(BeNil())
				})
			})

			When("scaling down from 2 to 1 instances", func() {
				BeforeEach(func() {
					instancesBefore = 2
					instancesAfter = 1
				})

				It("should delete the pod disruption budget for the lrp", func() {
					_, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, metav1.GetOptions{})
					Expect(err).To(MatchError(ContainSubstring("not found")))
				})
			})

			When("scaling down from 1 to 0 instances", func() {
				BeforeEach(func() {
					instancesBefore = 1
					instancesAfter = 0
				})

				It("should keep the lrp without a pod disruption budget", func() {
					_, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, metav1.GetOptions{})
					Expect(err).To(MatchError(ContainSubstring("not found")))
				})
			})
		})

		Describe("updating image", func() {
			var (
				imageBefore string
				imageAfter  string
				statefulset *appsv1.StatefulSet
			)

			BeforeEach(func() {
				imageBefore = "eirini/dorini"
				imageAfter = "eirini/notdora"
			})

			JustBeforeEach(func() {
				lrp.Image = imageBefore
				Expect(lrpClient.Desire(ctx, fixture.Namespace, lrp)).To(Succeed())

				lrp.Image = imageAfter
				Expect(lrpClient.Update(ctx, lrp)).To(Succeed())
				statefulset = getStatefulSetForLRP(lrp)
			})

			It("updates the image", func() {
				Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(Equal(imageAfter))
			})
		})
	})

	Describe("Get", func() {
		numberOfInstancesFn := func() int {
			actualLRP, err := lrpClient.Get(ctx, lrp.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())

			return actualLRP.RunningInstances
		}

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, lrp)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(numberOfInstancesFn).Should(Equal(lrp.TargetInstances))
			Consistently(numberOfInstancesFn, "10s").Should(Equal(lrp.TargetInstances))
		})

		When("one of the instances if failing", func() {
			BeforeEach(func() {
				lrp = createLRP("odin")
				lrp.Health = api.Healthcheck{
					Type: "port",
					Port: 3000,
				}
				lrp.Command = []string{
					"/bin/sh",
					"-c",
					`if [ $(echo $HOSTNAME | sed 's|.*-\(.*\)|\1|') -eq 1 ]; then
	exit;
else
	while true; do
		nc -lk -p 3000 -e echo just a server;
	done;
fi;`,
				}
			})

			It("correctly reports the running instances", func() {
				Eventually(numberOfInstancesFn).Should(Equal(1), fmt.Sprintf("pod %#v did not start", lrp.LRPIdentifier))
				Consistently(numberOfInstancesFn, "10s").Should(Equal(1), fmt.Sprintf("pod %#v did not keep running", lrp.LRPIdentifier))
			})
		})

		When("the LRP has 0 target instances", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 0
			})

			It("can still get the LRP", func() {
				actualLRP, err := lrpClient.Get(ctx, lrp.LRPIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRP.GUID).To(Equal(lrp.GUID))
			})
		})
	})

	Describe("GetInstances", func() {
		instancesFn := func() []*api.Instance {
			instances, err := lrpClient.GetInstances(ctx, lrp.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())

			return instances
		}

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, lrp)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(instancesFn).Should(HaveLen(lrp.TargetInstances))
			Consistently(instancesFn, "10s").Should(HaveLen(lrp.TargetInstances))
			Eventually(func() bool {
				instances := instancesFn()
				for _, instance := range instances {
					if instance.State != api.RunningState {
						return false
					}
				}

				return true
			}).Should(BeTrue())
		})

		When("the LRP has 0 target instances", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 0
			})

			It("returns an empty list", func() {
				Consistently(instancesFn, "10s").Should(BeEmpty())
			})
		})
	})

	Describe("List", func() {
		var listedLRPs []*api.LRP

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, lrp)
			Expect(err).ToNot(HaveOccurred())
			listedLRPs, err = lrpClient.List(context.Background())
			Expect(err).NotTo(HaveOccurred())
		})

		It("lists the lrps", func() {
			Expect(listedLRPs).To(HaveLen(1))
			Expect(listedLRPs[0].AppName).To(Equal("ödin"))
		})

		When("there are LRPs in foreign namespaces", func() {
			var extraNSClient *k8s.LRPClient

			BeforeEach(func() {
				extraNSClient = createLrpClient(fixture.CreateExtraNamespace(), false)
			})

			It("does not list LRPs in foreign namespaces", func() {
				extraListedLRPs, err := extraNSClient.List(context.Background())
				Expect(err).NotTo(HaveOccurred())
				Expect(extraListedLRPs).To(BeEmpty())
			})
		})

		When("non-eirini statefulSets exist", func() {
			var otherStatefulSetName string

			BeforeEach(func() {
				otherStatefulSetName = tests.GenerateGUID()

				_, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Create(context.Background(), &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: otherStatefulSetName,
					},
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"foo": "bar"},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
				}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not list them", func() {
				for _, lrp := range listedLRPs {
					Expect(lrp.GUID).NotTo(BeEmpty(), fmt.Sprintf("%#v does not look like an LRP", lrp))
					Expect(lrp.Version).NotTo(BeEmpty(), fmt.Sprintf("%#v does not look like an LRP", lrp))
				}
			})
		})
	})
})

func createLRP(name string) *api.LRP {
	return &api.LRP{
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		AppName:         name,
		AppGUID:         "the-app-guid",
		SpaceName:       "space-foo",
		TargetInstances: 2,
		Image:           "eirini/busybox",
		LRPIdentifier:   api.LRPIdentifier{GUID: tests.GenerateGUID(), Version: tests.GenerateGUID()},
		LRP:             "metadata",
		DiskMB:          2047,
		Env: map[string]string{
			"FOO": "BAR",
		},
	}
}

func createLrpClient(workloadsNamespace string, allowRunImageAsRoot bool) *k8s.LRPClient {
	logger := lagertest.NewTestLogger("test-" + workloadsNamespace)

	lrpToStatefulSetConverter := stset.NewLRPToStatefulSetConverter(
		tests.GetApplicationServiceAccount(),
		"registry-secret",
		false,
		allowRunImageAsRoot,
		123,
		k8s.CreateLivenessProbe,
		k8s.CreateReadinessProbe,
	)

	return k8s.NewLRPClient(
		logger,
		client.NewSecret(fixture.Clientset),
		client.NewStatefulSet(fixture.Clientset, workloadsNamespace),
		client.NewPod(fixture.Clientset, workloadsNamespace),
		pdb.NewUpdater(client.NewPodDisruptionBudget(fixture.Clientset)),
		client.NewEvent(fixture.Clientset),
		lrpToStatefulSetConverter,
		stset.NewStatefulSetToLRPConverter(),
	)
}

func int32ptr(i int) *int32 {
	i32 := int32(i)

	return &i32
}

func getPodPhase(index int, id api.LRPIdentifier) string {
	pod := listPods(id)[index]
	status := pod.Status

	if status.Phase != corev1.PodRunning {
		return fmt.Sprintf("Pod - %s", status.Phase)
	}

	if len(status.ContainerStatuses) == 0 {
		return "Containers status unknown"
	}

	for _, containerStatus := range status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			return fmt.Sprintf("Container %s - %v", containerStatus.Name, containerStatus.State)
		}

		if !containerStatus.Ready {
			return fmt.Sprintf("Container %s is not Ready", containerStatus.Name)
		}
	}

	return "Ready"
}

func getStatefulSetForLRP(lrp *api.LRP) *appsv1.StatefulSet {
	ss, getErr := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector(lrp.LRPIdentifier),
	})
	Expect(getErr).NotTo(HaveOccurred())
	Expect(ss.Items).To(HaveLen(1))

	return &ss.Items[0]
}
