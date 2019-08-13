package k8s_test

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/util/utilfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	testcore "k8s.io/client-go/testing"
)

const (
	namespace          = "testing"
	registrySecretName = "secret-name"
	rootfsVersion      = "version2"
)

var _ = Describe("Statefulset Desirer", func() {

	var (
		client                *fake.Clientset
		statefulSetDesirer    opi.Desirer
		livenessProbeCreator  *k8sfakes.FakeProbeCreator
		readinessProbeCreator *k8sfakes.FakeProbeCreator
		logger                *lagertest.TestLogger
	)

	listStatefulSets := func() []appsv1.StatefulSet {
		list, listErr := client.AppsV1().StatefulSets(namespace).List(meta.ListOptions{})
		Expect(listErr).NotTo(HaveOccurred())
		return list.Items
	}

	getStatefulSetFromK8s := func(lrp *opi.LRP) *appsv1.StatefulSet {
		labelSelector := fmt.Sprintf("guid=%s,version=%s", lrp.LRPIdentifier.GUID, lrp.LRPIdentifier.Version)
		ss, getErr := client.AppsV1().StatefulSets(namespace).List(meta.ListOptions{LabelSelector: labelSelector})
		Expect(getErr).NotTo(HaveOccurred())
		return &ss.Items[0]
	}

	BeforeEach(func() {
		client = fake.NewSimpleClientset()

		livenessProbeCreator = new(k8sfakes.FakeProbeCreator)
		readinessProbeCreator = new(k8sfakes.FakeProbeCreator)
		hasher := new(utilfakes.FakeHasher)
		hasher.HashReturns("random", nil)
		logger = lagertest.NewTestLogger("handler-test")
		statefulSetDesirer = &StatefulSetDesirer{
			Client:                client,
			Namespace:             namespace,
			RegistrySecretName:    registrySecretName,
			RootfsVersion:         rootfsVersion,
			LivenessProbeCreator:  livenessProbeCreator.Spy,
			ReadinessProbeCreator: readinessProbeCreator.Spy,
			Hasher:                hasher,
			Logger:                logger,
		}
	})

	Context("When creating an LRP", func() {
		Context("When app name only has [a-z0-9]", func() {
			var lrp *opi.LRP
			BeforeEach(func() {
				livenessProbeCreator.Returns(&corev1.Probe{})
				readinessProbeCreator.Returns(&corev1.Probe{})
				lrp = createLRP("Baldur", "my.example.route")
				Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())
			})

			It("should create a healthcheck probe", func() {
				Expect(livenessProbeCreator.CallCount()).To(Equal(1))
			})

			It("should create a readiness probe", func() {
				Expect(readinessProbeCreator.CallCount()).To(Equal(1))
			})

			It("should provide original request", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Annotations["original_request"]).To(Equal(lrp.LRP))
			})

			It("should provide the process-guid to the pod annotations", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Spec.Template.Annotations[cf.ProcessGUID]).To(Equal("Baldur-guid"))
			})

			It("should set name for the stateful set", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Name).To(Equal("baldur-space-foo-random"))
			})

			It("should set space name as annotation on the statefulset", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Annotations[cf.VcapSpaceName]).To(Equal("space-foo"))
			})

			It("should set seccomp pod annotation", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey]).To(Equal(corev1.SeccompProfileRuntimeDefault))
			})

			It("should set podManagementPolicy to parallel", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(string(statefulSet.Spec.PodManagementPolicy)).To(Equal("Parallel"))
			})

			It("should set podImagePullSecret", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
				secret := statefulSet.Spec.Template.Spec.ImagePullSecrets[0]
				Expect(secret.Name).To(Equal("secret-name"))
			})

			It("should deny privilegeEscalation", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(*statefulSet.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation).To(Equal(false))
			})

			It("should set imagePullPolicy to Always", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(string(statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy)).To(Equal("Always"))
			})

			It("should set rootfsVersion as a label", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, rootfsVersion))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, rootfsVersion))
			})

			It("should set disk limit", func() {
				statefulSet := getStatefulSetFromK8s(lrp)
				expectedLimit := resource.NewScaledQuantity(2048, resource.Mega)
				actualLimit := statefulSet.Spec.Template.Spec.Containers[0].Resources.Limits.StorageEphemeral()
				Expect(actualLimit).To(Equal(expectedLimit))
			})

			Context("When redeploying an existing LRP", func() {
				It("should fail", func() {
					newLrp := createLRP("Baldur", "my.example.route")
					Expect(statefulSetDesirer.Desire(newLrp)).To(MatchError(ContainSubstring("failed to create statefulset")))
				})
			})
		})

		Context("When the app name contains unsupported characters", func() {
			It("should use the guid as a name", func() {
				lrp := createLRP("Балдър", "my.example.route")
				Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())

				statefulSet := getStatefulSetFromK8s(lrp)
				Expect(statefulSet.Name).To(Equal("guid_1234-random"))
			})
		})
	})

	Context("When getting an app", func() {
		It("return the same LRP except metadata and original LRP request", func() {
			expectedLRP := createLRP("Baldur", "my.example.route")
			Expect(statefulSetDesirer.Desire(expectedLRP)).To(Succeed())
			expectedLRP.Metadata = cleanupMetadata(expectedLRP.Metadata)
			expectedLRP.LRP = ""

			Expect(statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Equal(expectedLRP))
		})

		Context("when the app does not exist", func() {
			It("should return an error", func() {
				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("statefulset not found")))
			})
		})
	})

	Context("When updating an app", func() {
		Context("when the app exists", func() {
			var (
				lrp *opi.LRP
			)

			BeforeEach(func() {
				lrp = createLRP("update", `["my.example.route"]`)
				Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())
			})

			Context("when update fails", func() {

				BeforeEach(func() {
					reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("boom")
					}
					client.PrependReactor("update", "statefulsets", reaction)
				})

				It("should return a meaningful message", func() {
					Expect(statefulSetDesirer.Update(lrp)).To(MatchError(ContainSubstring("failed to update statefulset")))
				})

			})

			Context("with replica count modified", func() {
				It("only updates the desired number of app instances and last updated", func() {
					lrp.TargetInstances = 5
					lrp.Metadata[cf.LastUpdated] = "never"

					originalStatefulSet := getStatefulSetFromK8s(lrp)

					Expect(statefulSetDesirer.Update(lrp)).To(Succeed())

					Eventually(func() int32 {
						return *getStatefulSetFromK8s(lrp).Spec.Replicas
					}).Should(Equal(int32(5)))

					newAnnotations := getStatefulSetFromK8s(lrp).GetAnnotations()
					Expect(newAnnotations[cf.LastUpdated]).To(Equal("never"))
					originalAnnotations := originalStatefulSet.GetAnnotations()
					delete(originalAnnotations, cf.LastUpdated)
					delete(newAnnotations, cf.LastUpdated)
					Expect(originalAnnotations).To(Equal(newAnnotations))
				})
			})

			Context("with modified routes", func() {
				It("only updates the stored routes", func() {
					originalStatefulSet := getStatefulSetFromK8s(lrp)
					lrp.Metadata = map[string]string{cf.VcapAppUris: `["my.example.route", "my.second.example.route"]`, cf.LastUpdated: "yes"}
					Expect(statefulSetDesirer.Update(lrp)).To(Succeed())

					Eventually(func() string {
						return getStatefulSetFromK8s(lrp).Annotations[eirini.RegisteredRoutes]
					}, 1*time.Second).Should(Equal(`["my.example.route", "my.second.example.route"]`))

					newAnnotations := getStatefulSetFromK8s(lrp).GetAnnotations()
					Expect(newAnnotations[cf.LastUpdated]).To(Equal("yes"))

					originalAnnotations := originalStatefulSet.GetAnnotations()
					delete(originalAnnotations, eirini.RegisteredRoutes)
					delete(originalAnnotations, cf.LastUpdated)
					delete(newAnnotations, eirini.RegisteredRoutes)
					delete(newAnnotations, cf.LastUpdated)
					Expect(originalAnnotations).To(Equal(newAnnotations))
				})
			})

		})

		Context("when the app does not exist", func() {
			It("should return an error", func() {
				Expect(statefulSetDesirer.Update(createLRP("name", "[something.strange]"))).
					To(MatchError(ContainSubstring("failed to get statefulset")))
			})

			It("should not create the app", func() {
				Expect(statefulSetDesirer.Update(createLRP("name", "[something.strange]"))).
					To(HaveOccurred())
				sets, listErr := client.AppsV1().StatefulSets(namespace).List(meta.ListOptions{})
				Expect(listErr).NotTo(HaveOccurred())
				Expect(sets.Items).To(BeEmpty())
			})

		})
	})

	Context("When listing apps", func() {
		It("translates all existing statefulSets to opi.LRPs", func() {
			expectedLRPs := []*opi.LRP{
				createLRP("odin", "my.example.route"),
				createLRP("thor", "my.example.route"),
				createLRP("mimir", "my.example.route"),
			}

			for _, l := range expectedLRPs {
				Expect(statefulSetDesirer.Desire(l)).To(Succeed())
			}
			// clean metadata and LRP because we do not return LRP
			// and return only subset of metadata fields
			for _, l := range expectedLRPs {
				l.Metadata = cleanupMetadata(l.Metadata)
				l.LRP = ""
			}

			Expect(statefulSetDesirer.List()).To(ConsistOf(expectedLRPs))
		})

		Context("no statefulSets exist", func() {
			It("returns an empy list of LRPs", func() {
				Expect(statefulSetDesirer.List()).To(BeEmpty())
			})
		})

		Context("fails to list the statefulsets", func() {

			It("should return a meaningful error", func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("list", "statefulsets", reaction)

				_, err := statefulSetDesirer.List()
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})

		})
	})

	Context("Stop an LRP", func() {

		BeforeEach(func() {
			lrp := createLRP("Baldur", "my.example.route")
			Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())
		})

		Context("Successful stop", func() {
			It("deletes the statefulSet", func() {
				Expect(listStatefulSets()).NotTo(BeEmpty())
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())

				Expect(listStatefulSets()).To(BeEmpty())
			})

			It("deletes the associated LRP jobs", func() {
				job := createJob("guid_1234")
				_, err := client.BatchV1().Jobs(namespace).Create(job)
				Expect(err).NotTo(HaveOccurred())

				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				jobs, err := client.BatchV1().Jobs(namespace).List(meta.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(jobs.Items).To(BeEmpty())
			})
		})

		Context("when deletion of job fails", func() {
			It("should return a meaningful error", func() {
				job := createJob("guid_1234")
				_, err := client.BatchV1().Jobs(namespace).Create(job)
				Expect(err).NotTo(HaveOccurred())
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("delete", "jobs", reaction)
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to delete job")))
			})
		})

		Context("when deletion of stateful set fails", func() {
			It("should return a meaningful error", func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("delete", "statefulsets", reaction)
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to delete statefulset")))
			})
		})

		Context("when kubernetes fails to list jobs", func() {
			It("should return a meaningful error", func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("list", "jobs", reaction)
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to list jobs")))
			})
		})

		Context("when kubernetes fails to list statefulsets", func() {
			It("should return a meaningful error", func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("list", "statefulsets", reaction)
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})

		})

		Context("when the statefulSet does not exist", func() {
			It("returns success", func() {
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).
					To(Succeed())
			})

			It("logs useful information", func() {
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "missing_guid", Version: "some_version"})).To(Succeed())
				Expect(logger).To(gbytes.Say("missing_guid"))
			})
		})
	})

	Context("Stop an LRP instance", func() {

		BeforeEach(func() {
			lrp := createLRP("Baldur", "my.example.route")
			Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())

			pod0 := toPod("baldur-space-foo-random", 0, nil)
			_, err := client.CoreV1().Pods(namespace).Create(pod0)
			Expect(err).ToNot(HaveOccurred())

			pod1 := toPod("baldur-space-foo-random", 1, nil)
			_, err = client.CoreV1().Pods(namespace).Create(pod1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes a pod instance", func() {
			Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
				To(Succeed())

			pods, err := client.CoreV1().Pods(namespace).List(meta.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].Name).To(Equal("baldur-space-foo-random-0"))
		})

		Context("when there's an internal K8s error", func() {

			It("should return an error", func() {

				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("list", "statefulsets", reaction)

				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(MatchError("failed to get statefulset: boom"))
			})
		})

		Context("when the statefulset does not exist", func() {

			It("returns an error", func() {
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "some", Version: "thing"}, 1)).
					To(MatchError("app does not exist"))
			})
		})

		Context("when the instance does not exist", func() {

			It("returns an error", func() {
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 42)).
					To(MatchError(ContainSubstring("failed to delete pod")))
			})
		})
	})

	Context("Get LRP instances", func() {

		BeforeEach(func() {
			since1 := meta.Unix(123, 0)
			pod1 := toPod("odin", 0, &since1)
			since2 := meta.Unix(456, 0)
			pod2 := toPod("odin", 1, &since2)
			_, err := client.CoreV1().Pods(namespace).Create(pod1)
			Expect(err).ToNot(HaveOccurred())

			_, err = client.CoreV1().Pods(namespace).Create(pod2)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the correct number of instances", func() {
			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(2))
			Expect(instances[0]).To(Equal(toInstance(0, 123000000000)))
			Expect(instances[1]).To(Equal(toInstance(1, 456000000000)))
		})

		Context("when pod list fails", func() {

			It("should return a meaningful error", func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("list", "pods", reaction)

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).To(MatchError(ContainSubstring("failed to list pods")))
			})

		})

		Context("when getting events fails", func() {

			It("should return a meaningful error", func() {
				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("boom")
				}
				client.PrependReactor("list", "events", reaction)

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("failed to get events for pod %s", "odin-0"))))
			})

		})

		Context("and time since creation is not available yet", func() {

			It("should return a default value", func() {
				pod3 := toPod("mimir", 2, nil)
				_, err := client.CoreV1().Pods(namespace).Create(pod3)
				Expect(err).ToNot(HaveOccurred())

				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(3))
				Expect(instances[2]).To(Equal(toInstance(2, 0)))
			})
		})

		Context("and the StatefulSet was deleted/stopped", func() {

			It("should return a default value", func() {
				event1 := &corev1.Event{
					Reason: "Killing",
					InvolvedObject: corev1.ObjectReference{
						Name:      "odin-0",
						Namespace: namespace,
						UID:       "odin-0-uid",
					},
				}
				event2 := &corev1.Event{
					Reason: "Killing",
					InvolvedObject: corev1.ObjectReference{
						Name:      "odin-1",
						Namespace: namespace,
						UID:       "odin-1-uid",
					},
				}

				event1.Name = "event1"
				event2.Name = "event2"

				_, clientErr := client.CoreV1().Events(namespace).Create(event1)
				Expect(clientErr).ToNot(HaveOccurred())
				_, clientErr = client.CoreV1().Events(namespace).Create(event2)
				Expect(clientErr).ToNot(HaveOccurred())

				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(0))
			})
		})

	})
})

func toPod(lrpName string, index int, time *meta.Time) *corev1.Pod {
	pod := corev1.Pod{}
	pod.Name = lrpName + "-" + strconv.Itoa(index)
	pod.UID = types.UID(pod.Name + "-uid")
	pod.Labels = map[string]string{
		"guid":    "guid_1234",
		"version": "version_1234",
	}

	pod.Status.StartTime = time
	pod.Status.Phase = corev1.PodRunning
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			Ready: true,
		},
	}
	return &pod
}

func toInstance(index int, since int64) *opi.Instance {
	return &opi.Instance{
		Index: index,
		Since: since,
		State: "RUNNING",
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytes() string {
	b := make([]byte, 10)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func createLRP(name, routes string) *opi.LRP {
	lastUpdated := randStringBytes()
	return &opi.LRP{
		LRPIdentifier: opi.LRPIdentifier{
			GUID:    "guid_1234",
			Version: "version_1234",
		},
		AppName:   name,
		SpaceName: "space-foo",
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		RunningInstances: 0,
		MemoryMB:         1024,
		DiskMB:           2048,
		Image:            "busybox",
		Ports:            []int32{8888, 9999},
		Metadata: map[string]string{
			cf.ProcessGUID: name + "-guid",
			cf.LastUpdated: lastUpdated,
			cf.VcapAppUris: routes,
			cf.VcapAppName: name,
			cf.VcapAppID:   "guid_1234",
			cf.VcapVersion: "version_1234",
		},
		VolumeMounts: []opi.VolumeMount{
			{
				ClaimName: "some-claim",
				MountPath: "/some/path",
			},
		},
		LRP: "original request",
	}
}

func createJob(guid string) *batch.Job {
	job := &batch.Job{
		Spec: batch.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	job.Name = guid

	labels := map[string]string{
		"guid": guid,
	}

	job.Spec.Template.Labels = labels
	job.Labels = labels
	return job
}

func cleanupMetadata(m map[string]string) map[string]string {
	var fields = []string{
		"process_guid",
		"last_updated",
		"application_uris",
		"application_id",
		"version",
		"application_name",
	}

	result := map[string]string{}
	for _, f := range fields {
		result[f] = m[f]
	}
	return result
}
