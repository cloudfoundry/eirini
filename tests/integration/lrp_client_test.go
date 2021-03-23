package integration_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/pdb"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("LRPClient", func() {
	var (
		allowRunImageAsRoot bool
		lrpClient           *k8s.LRPClient
		odinLRP             *opi.LRP
		thorLRP             *opi.LRP
	)

	BeforeEach(func() {
		allowRunImageAsRoot = false
		odinLRP = createLRP("ödin")
		thorLRP = createLRP("thor")
	})

	AfterEach(func() {
		cleanupStatefulSet(odinLRP)
		cleanupStatefulSet(thorLRP)
		Eventually(func() []appsv1.StatefulSet {
			return listAllStatefulSets(odinLRP, thorLRP)
		}).Should(BeEmpty())
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("test")

		lrpToStatefulSetConverter := stset.NewLRPToStatefulSetConverter(
			tests.GetApplicationServiceAccount(),
			"registry-secret",
			false,
			allowRunImageAsRoot,
			1,
			k8s.CreateLivenessProbe,
			k8s.CreateReadinessProbe,
		)
		lrpClient = k8s.NewLRPClient(
			logger,
			client.NewSecret(fixture.Clientset),
			client.NewStatefulSet(fixture.Clientset, fixture.Namespace),
			client.NewPod(fixture.Clientset, fixture.Namespace),
			pdb.NewCreatorDeleter(client.NewPodDisruptionBudget(fixture.Clientset)),
			client.NewEvent(fixture.Clientset),
			lrpToStatefulSetConverter,
			stset.NewStatefulSetToLRPConverter(),
		)
	})

	Describe("Desire", func() {
		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, odinLRP)
			Expect(err).ToNot(HaveOccurred())
			err = lrpClient.Desire(ctx, fixture.Namespace, thorLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		// join all tests in a single with By()
		It("should create a StatefulSet object", func() {
			statefulset := getStatefulSetForLRP(odinLRP)
			Expect(statefulset.Name).To(ContainSubstring(odinLRP.GUID))
			Expect(statefulset.Namespace).To(Equal(fixture.Namespace))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Command).To(Equal(odinLRP.Command))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(Equal(odinLRP.Image))
			Expect(statefulset.Spec.Replicas).To(Equal(int32ptr(odinLRP.TargetInstances)))
			Expect(statefulset.Annotations[stset.AnnotationOriginalRequest]).To(Equal(odinLRP.LRP))
		})

		It("should create all associated pods", func() {
			var podNames []string

			Eventually(func() []string {
				podNames = podNamesFromPods(listPods(odinLRP.LRPIdentifier))

				return podNames
			}).Should(HaveLen(odinLRP.TargetInstances))

			for i := 0; i < odinLRP.TargetInstances; i++ {
				podIndex := i
				Expect(podNames[podIndex]).To(ContainSubstring(odinLRP.GUID))

				Eventually(func() string {
					return getPodPhase(podIndex, odinLRP.LRPIdentifier)
				}).Should(Equal("Ready"))
			}

			statefulset := getStatefulSetForLRP(odinLRP)
			Eventually(func() int32 {
				return getStatefulSetForLRP(odinLRP).Status.ReadyReplicas
			}).Should(Equal(*statefulset.Spec.Replicas))
		})

		It("should create a pod disruption budget for the lrp", func() {
			statefulset := getStatefulSetForLRP(odinLRP)
			pdb, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, v1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pdb).NotTo(BeNil())
		})

		Context("when the lrp has 1 instance", func() {
			BeforeEach(func() {
				odinLRP.TargetInstances = 1
			})

			It("should not create a pod disruption budget for the lrp", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				_, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, v1.GetOptions{})
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})

		Context("when additional app info is provided", func() {
			BeforeEach(func() {
				odinLRP.OrgName = "odin-org"
				odinLRP.OrgGUID = "odin-org-guid"
				odinLRP.SpaceName = "odin-space"
				odinLRP.SpaceGUID = "odin-space-guid"
			})

			DescribeTable("sets appropriate annotations to statefulset", func(key, value string) {
				statefulset := getStatefulSetForLRP(odinLRP)
				Expect(statefulset.Annotations).To(HaveKeyWithValue(key, value))
			},
				Entry("SpaceName", stset.AnnotationSpaceName, "odin-space"),
				Entry("SpaceGUID", stset.AnnotationSpaceGUID, "odin-space-guid"),
				Entry("OrgName", stset.AnnotationOrgName, "odin-org"),
				Entry("OrgGUID", stset.AnnotationOrgGUID, "odin-org-guid"),
			)

			It("sets appropriate labels to statefulset", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				Expect(statefulset.Labels).To(HaveKeyWithValue(stset.LabelGUID, odinLRP.LRPIdentifier.GUID))
				Expect(statefulset.Labels).To(HaveKeyWithValue(stset.LabelVersion, odinLRP.LRPIdentifier.Version))
				Expect(statefulset.Labels).To(HaveKeyWithValue(stset.LabelSourceType, "APP"))
			})
		})

		Context("when the app has more than one instances", func() {
			BeforeEach(func() {
				odinLRP.TargetInstances = 2
			})

			It("should schedule app pods on different nodes", func() {
				if getNodeCount() == 1 {
					Skip("target cluster has only one node")
				}

				Eventually(func() []corev1.Pod {
					return listPods(odinLRP.LRPIdentifier)
				}).Should(HaveLen(2))

				var nodeNames []string
				Eventually(func() []string {
					nodeNames = nodeNamesFromPods(listPods(odinLRP.LRPIdentifier))

					return nodeNames
				}).Should(HaveLen(2))
				Expect(nodeNames[0]).ToNot(Equal(nodeNames[1]))
			})
		})

		Context("When private docker registry credentials are provided", func() {
			BeforeEach(func() {
				odinLRP.Image = "eiriniuser/notdora:latest"
				odinLRP.Command = nil
				odinLRP.PrivateRegistry = &opi.PrivateRegistry{
					Server:   "index.docker.io/v1/",
					Username: "eiriniuser",
					Password: tests.GetEiriniDockerHubPassword(),
				}
			})

			It("creates a private registry secret", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				secret, err := getSecret(fixture.Namespace, privateRegistrySecretName(statefulset.Name))
				Expect(err).NotTo(HaveOccurred())
				Expect(secret).NotTo(BeNil())
			})

			It("sets the ImagePullSecret correctly in the pod template", func() {
				Eventually(func() []corev1.Pod {
					return listPods(odinLRP.LRPIdentifier)
				}).Should(HaveLen(odinLRP.TargetInstances))

				for i := 0; i < odinLRP.TargetInstances; i++ {
					podIndex := i
					Eventually(func() string {
						return getPodPhase(podIndex, odinLRP.LRPIdentifier)
					}).Should(Equal("Ready"))
				}
			})
		})

		Context("when we create the same StatefulSet again", func() {
			It("should not error", func() {
				err := lrpClient.Desire(ctx, fixture.Namespace, odinLRP)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("When using a docker image that needs root access", func() {
			BeforeEach(func() {
				allowRunImageAsRoot = true

				odinLRP.Image = "eirini/nginx-integration"
				odinLRP.Command = nil
				odinLRP.Health.Type = "http"
				odinLRP.Health.Port = 8080
			})

			It("should start all the pods", func() {
				var podNames []string

				Eventually(func() []string {
					podNames = podNamesFromPods(listPods(odinLRP.LRPIdentifier))

					return podNames
				}).Should(HaveLen(odinLRP.TargetInstances))

				for i := 0; i < odinLRP.TargetInstances; i++ {
					podIndex := i
					Eventually(func() string {
						return getPodPhase(podIndex, odinLRP.LRPIdentifier)
					}).Should(Equal("Ready"))
				}

				Eventually(func() int32 {
					statefulset := getStatefulSetForLRP(odinLRP)

					return statefulset.Status.ReadyReplicas
				}).Should(BeNumerically("==", odinLRP.TargetInstances))
			})
		})

		Context("when the LRP has 0 target instances", func() {
			BeforeEach(func() {
				odinLRP.TargetInstances = 0
			})

			It("still creates a statefulset, with 0 replicas", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				Expect(statefulset.Name).To(ContainSubstring(odinLRP.GUID))
				Expect(statefulset.Spec.Replicas).To(Equal(int32ptr(0)))
			})
		})
	})

	Describe("Stop", func() {
		var statefulsetName string

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, odinLRP)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []corev1.Pod {
				return listPods(odinLRP.LRPIdentifier)
			}).Should(HaveLen(odinLRP.TargetInstances))

			statefulsetName = getStatefulSetForLRP(odinLRP).Name

			err = lrpClient.Stop(ctx, odinLRP.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the StatefulSet object", func() {
			Eventually(func() []appsv1.StatefulSet {
				return listStatefulSetsForApp("odin")
			}).Should(BeEmpty())
		})

		It("should delete the associated pods", func() {
			Eventually(func() []corev1.Pod {
				return listPods(odinLRP.LRPIdentifier)
			}).Should(BeEmpty())
		})

		It("should delete the pod disruption budget for the lrp", func() {
			Eventually(func() error {
				_, err := podDisruptionBudgets().Get(context.Background(), statefulsetName, v1.GetOptions{})

				return err
			}).Should(MatchError(ContainSubstring("not found")))
		})

		Context("when the lrp has only 1 instance", func() {
			BeforeEach(func() {
				odinLRP.TargetInstances = 1
			})

			It("keep the lrp without a pod disruption budget", func() {
				Eventually(func() error {
					_, err := podDisruptionBudgets().Get(context.Background(), statefulsetName, v1.GetOptions{})

					return err
				}).Should(MatchError(ContainSubstring("not found")))
			})
		})

		Context("When private docker registry credentials are provided", func() {
			BeforeEach(func() {
				odinLRP.Image = "eiriniuser/notdora:latest"
				odinLRP.PrivateRegistry = &opi.PrivateRegistry{
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
				_, err := getSecret(fixture.Namespace, privateRegistrySecretName(statefulsetName))
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})
	})

	Describe("Update", func() {
		var (
			instancesBefore int
			instancesAfter  int
		)

		JustBeforeEach(func() {
			odinLRP.TargetInstances = instancesBefore
			Expect(lrpClient.Desire(ctx, fixture.Namespace, odinLRP)).To(Succeed())

			odinLRP.TargetInstances = instancesAfter
			Expect(lrpClient.Update(ctx, odinLRP)).To(Succeed())
		})

		Context("when scaling up from 1 to 2 instances", func() {
			BeforeEach(func() {
				instancesBefore = 1
				instancesAfter = 2
			})

			It("should create a pod disruption budget for the lrp", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				pdb, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, v1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pdb).NotTo(BeNil())
			})
		})

		Context("when scaling up from 2 to 3 instances", func() {
			BeforeEach(func() {
				instancesBefore = 2
				instancesAfter = 3
			})

			It("should keep the existing pod disruption budget for the lrp", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				pdb, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, v1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pdb).NotTo(BeNil())
			})
		})

		Context("when scaling down from 2 to 1 instances", func() {
			BeforeEach(func() {
				instancesBefore = 2
				instancesAfter = 1
			})

			It("should delete the pod disruption budget for the lrp", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				_, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, v1.GetOptions{})
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})

		Context("when scaling down from 1 to 0 instances", func() {
			BeforeEach(func() {
				instancesBefore = 1
				instancesAfter = 0
			})

			It("should keep the lrp without a pod disruption budget", func() {
				statefulset := getStatefulSetForLRP(odinLRP)
				_, err := podDisruptionBudgets().Get(context.Background(), statefulset.Name, v1.GetOptions{})
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})
	})

	Describe("Get", func() {
		numberOfInstancesFn := func() int {
			lrp, err := lrpClient.Get(ctx, odinLRP.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())

			return lrp.RunningInstances
		}

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, odinLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(numberOfInstancesFn).Should(Equal(odinLRP.TargetInstances))
			Consistently(numberOfInstancesFn, "10s").Should(Equal(odinLRP.TargetInstances))
		})

		Context("When one of the instances if failing", func() {
			BeforeEach(func() {
				odinLRP = createLRP("odin")
				odinLRP.Health = opi.Healtcheck{
					Type: "port",
					Port: 3000,
				}
				odinLRP.Command = []string{
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
				Eventually(numberOfInstancesFn).Should(Equal(1), fmt.Sprintf("pod %#v did not start", odinLRP.LRPIdentifier))
				Consistently(numberOfInstancesFn, "10s").Should(Equal(1), fmt.Sprintf("pod %#v did not keep running", odinLRP.LRPIdentifier))
			})
		})

		Context("when the LRP has 0 target instances", func() {
			BeforeEach(func() {
				odinLRP.TargetInstances = 0
			})

			It("can still get the LRP", func() {
				lrp, err := lrpClient.Get(ctx, odinLRP.LRPIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(lrp.GUID).To(Equal(odinLRP.GUID))
			})
		})
	})

	Describe("GetInstances", func() {
		instancesFn := func() []*opi.Instance {
			instances, err := lrpClient.GetInstances(ctx, odinLRP.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())

			return instances
		}

		JustBeforeEach(func() {
			err := lrpClient.Desire(ctx, fixture.Namespace, odinLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(instancesFn).Should(HaveLen(odinLRP.TargetInstances))
			Consistently(instancesFn, "10s").Should(HaveLen(odinLRP.TargetInstances))
			Eventually(func() bool {
				instances := instancesFn()
				for _, instance := range instances {
					if instance.State != opi.RunningState {
						return false
					}
				}

				return true
			}).Should(BeTrue())
		})

		Context("when the LRP has 0 target instances", func() {
			BeforeEach(func() {
				odinLRP.TargetInstances = 0
			})

			It("returns an empty list", func() {
				Consistently(instancesFn, "10s").Should(BeEmpty())
			})
		})
	})
})

func int32ptr(i int) *int32 {
	i32 := int32(i)

	return &i32
}

func getPodPhase(index int, id opi.LRPIdentifier) string {
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

func privateRegistrySecretName(statefulsetName string) string {
	return fmt.Sprintf("%s-registry-credentials", statefulsetName)
}
