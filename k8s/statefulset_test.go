package k8s_test

// import (
// 	"errors"
// 	"fmt"
// 	"math/rand"
// 	"strconv"

// 	"code.cloudfoundry.org/eirini"
// 	. "code.cloudfoundry.org/eirini/k8s"
// 	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
// 	"code.cloudfoundry.org/eirini/models/cf"
// 	"code.cloudfoundry.org/eirini/opi"
// 	"code.cloudfoundry.org/eirini/rootfspatcher"
// 	"code.cloudfoundry.org/eirini/util/utilfakes"
// 	"code.cloudfoundry.org/lager/lagertest"
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// 	"github.com/onsi/gomega/gbytes"
// 	appsv1 "k8s.io/api/apps/v1"
// 	corev1 "k8s.io/api/core/v1"

// 	"k8s.io/apimachinery/pkg/api/resource"
// 	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/types"
// 	"k8s.io/client-go/kubernetes/fake"
// 	_ "k8s.io/client-go/plugin/pkg/client/auth"
// 	testcore "k8s.io/client-go/testing"
// )

// const (
// 	namespace          = "testing"
// 	registrySecretName = "secret-name"
// 	rootfsVersion      = "version2"
// )

// var _ = XDescribe("Statefulset Desirer", func() {

// 	var (
// 		client                *fake.Clientset
// 		podClient             *k8sfakes.FakePodListerDeleter
// 		statefulSetClient     *k8sfakes.FakeStatefulSetClient
// 		statefulSetDesirer    opi.Desirer
// 		livenessProbeCreator  *k8sfakes.FakeProbeCreator
// 		readinessProbeCreator *k8sfakes.FakeProbeCreator
// 		logger                *lagertest.TestLogger
// 	)

// 	listStatefulSets := func() []appsv1.StatefulSet {
// 		list, listErr := client.AppsV1().StatefulSets(namespace).List(meta.ListOptions{})
// 		Expect(listErr).NotTo(HaveOccurred())
// 		return list.Items
// 	}

// 	BeforeEach(func() {
// 		client = fake.NewSimpleClientset()
// 		podClient = new(k8sfakes.FakePodListerDeleter)
// 		statefulSetClient = new(k8sfakes.FakeStatefulSetClient)

// 		livenessProbeCreator = new(k8sfakes.FakeProbeCreator)
// 		readinessProbeCreator = new(k8sfakes.FakeProbeCreator)
// 		hasher := new(utilfakes.FakeHasher)
// 		hasher.HashReturns("random", nil)
// 		logger = lagertest.NewTestLogger("handler-test")
// 		statefulSetDesirer = &StatefulSetDesirer{
// 			Client:                client,
// 			Pods:                  podClient,
// 			StatefulSets:          statefulSetClient,
// 			Namespace:             namespace,
// 			RegistrySecretName:    registrySecretName,
// 			RootfsVersion:         rootfsVersion,
// 			LivenessProbeCreator:  livenessProbeCreator.Spy,
// 			ReadinessProbeCreator: readinessProbeCreator.Spy,
// 			Hasher:                hasher,
// 			Logger:                logger,
// 		}
// 	})
// 	}

// 	Context("When creating an LRP", func() {
// 		Context("When app name only has [a-z0-9]", func() {

// 			var lrp *opi.LRP

// 			BeforeEach(func() {
// 				livenessProbeCreator.Returns(&corev1.Probe{})
// 				readinessProbeCreator.Returns(&corev1.Probe{})
// 				lrp = createLRP("Baldur", "my.example.route")
// 				Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())
// 			})

// 			It("should call the statefulset client", func() {
// 				Expect(statefulSetClient.CreateCallCount()).To(Equal(1))
// 			})

// 			It("should create a healthcheck probe", func() {
// 				Expect(livenessProbeCreator.CallCount()).To(Equal(1))
// 			})

// 			It("should create a readiness probe", func() {
// 				Expect(readinessProbeCreator.CallCount()).To(Equal(1))
// 			})

// 			It("should provide original request", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Annotations["original_request"]).To(Equal(lrp.LRP))
// 			})

// 			It("should provide the process-guid to the pod annotations", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Spec.Template.Annotations[cf.ProcessGUID]).To(Equal("Baldur-guid"))
// 			})

// 			It("should set name for the stateful set", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Name).To(Equal("baldur-space-foo-random"))
// 			})

// 			It("should set space name as annotation on the statefulset", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Annotations[cf.VcapSpaceName]).To(Equal("space-foo"))
// 			})

// 			It("should set seccomp pod annotation", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey]).To(Equal(corev1.SeccompProfileRuntimeDefault))
// 			})

// 			It("should set podManagementPolicy to parallel", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(string(statefulSet.Spec.PodManagementPolicy)).To(Equal("Parallel"))
// 			})

// 			It("should set podImagePullSecret", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
// 				secret := statefulSet.Spec.Template.Spec.ImagePullSecrets[0]
// 				Expect(secret.Name).To(Equal("secret-name"))
// 			})

// 			It("should deny privilegeEscalation", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(*statefulSet.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation).To(Equal(false))
// 			})

// 			It("should set imagePullPolicy to Always", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(string(statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy)).To(Equal("Always"))
// 			})

// 			It("should set rootfsVersion as a label", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				Expect(statefulSet.Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, rootfsVersion))
// 				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, rootfsVersion))
// 			})

// 			It("should set disk limit", func() {
// 				statefulSet := statefulSetClient.CreateArgsForCall(0)

// 				expectedLimit := resource.NewScaledQuantity(2048, resource.Mega)
// 				actualLimit := statefulSet.Spec.Template.Spec.Containers[0].Resources.Limits.StorageEphemeral()
// 				Expect(actualLimit).To(Equal(expectedLimit))
// 			})
// 		})

// 		Context("When the app name contains unsupported characters", func() {
// 			It("should use the guid as a name", func() {
// 				lrp := createLRP("Балдър", "my.example.route")
// 				Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())

// 				statefulSet := statefulSetClient.CreateArgsForCall(0)
// 				Expect(statefulSet.Name).To(Equal("guid_1234-random"))
// 			})
// 		})
// 	})

// 	FContext("When getting an app", func() {

// 		BeforeEach(func() {
// 			st := &appsv1.StatefulSetList{
// 				Items: []appsv1.StatefulSet{
// 					{
// 						ObjectMeta: meta.ObjectMeta{
// 							Name: "baldur",
// 							Annotations: map[string]string{
// 								cf.ProcessGUID:   "Baldur-guid",
// 								cf.LastUpdated:   "last-updated-some-time-ago",
// 								cf.VcapAppUris:   "my.example.route",
// 								cf.VcapAppID:     "guid_1234",
// 								cf.VcapVersion:   "version_1234",
// 								cf.VcapAppName:   "Baldur",
// 								cf.VcapSpaceName: "space-foo",
// 							},
// 						},
// 						Spec: appsv1.StatefulSetSpec{
// 							Template: corev1.PodTemplateSpec{
// 								Spec: corev1.PodSpec{
// 									Containers: []corev1.Container{
// 										{
// 											Image: "busybox",
// 											Command: []string{
// 												"/bin/sh",
// 												"-c",
// 												"while true; do echo hello; sleep 10;done",
// 											},
// 											Ports: []corev1.ContainerPort{
// 												{
// 													ContainerPort: 8888,
// 												},
// 												{
// 													ContainerPort: 9999,
// 												},
// 											},
// 											Resources: corev1.ResourceRequirements{
// 												Requests: corev1.ResourceList{
// 													corev1.ResourceMemory: *resource.NewScaledQuantity(1024, resource.Mega),
// 												},
// 												Limits: corev1.ResourceList{
// 													corev1.ResourceEphemeralStorage: *resource.NewScaledQuantity(2048, resource.Mega),
// 												},
// 											},
// 											VolumeMounts: []corev1.VolumeMount{
// 												{
// 													Name:      "some-claim",
// 													MountPath: "/some/path",
// 												},
// 											},
// 										},
// 									},
// 								},
// 							},
// 						},
// 						Status: appsv1.StatefulSetStatus{
// 							ReadyReplicas: 3,
// 						},
// 					},
// 				},
// 			}

// 			statefulSetClient.ListReturns(st, nil)
// 		})
// 		It("should set the correct LRP identifier", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
// 			Expect(lrp.LRPIdentifier.GUID).To(Equal("guid_1234"))
// 			Expect(lrp.LRPIdentifier.Version).To(Equal("version_1234"))
// 		})

// 		It("should set the correct LRP app name", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
// 			Expect(lrp.AppName).To(Equal("Baldur"))
// 		})

// 		It("should set the correct LRP space name", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
// 			Expect(lrp.SpaceName).To(Equal("space-foo"))
// 		})

// 		It("should set the correct LRP image", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
// 			Expect(lrp.Image).To(Equal("busybox"))
// 		})

// 		It("should set the correct LRP command", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(lrp.Command).To(Equal([]string{"/bin/sh", "-c", "while true; do echo hello; sleep 10;done"}))
// 		})

// 		It("should set the correct LRP running instances", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(lrp.RunningInstances).To(Equal(3))
// 		})

// 		It("should set the correct LRP ports", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(lrp.Ports).To(Equal([]int32{8888, 9999}))
// 		})

// 		It("should translate statefulset annotations to LRP metadata", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
// 			Expect(lrp.Metadata).To(Equal(map[string]string{
// 				cf.ProcessGUID: "Baldur-guid",
// 				cf.LastUpdated: "last-updated-some-time-ago",
// 				cf.VcapAppUris: "my.example.route",
// 				cf.VcapAppID:   "guid_1234",
// 				cf.VcapVersion: "version_1234",
// 				cf.VcapAppName: "Baldur",
// 			}))
// 		})

// 		It("should set the correct LRP disk and memory usage", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(lrp.MemoryMB).To(Equal(int64(1024)))
// 			Expect(lrp.DiskMB).To(Equal(int64(2048)))
// 		})

// 		It("should set the correct LRP volume mounts", func() {
// 			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(lrp.VolumeMounts).To(Equal([]opi.VolumeMount{
// 				{
// 					ClaimName: "some-claim",
// 					MountPath: "/some/path",
// 				},
// 			}))
// 		})

// 		Context("when the app does not exist", func() {

// 			BeforeEach(func() {
// 				statefulSetClient.ListReturns(&appsv1.StatefulSetList{}, nil)
// 			})

// 			It("should return an error", func() {
// 				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
// 				Expect(err).To(MatchError(ContainSubstring("statefulset not found")))
// 			})
// 		})

// 		Context("when statefulsets cannot be listed", func() {

// 			BeforeEach(func() {
// 				statefulSetClient.ListReturns(nil, errors.New("who is this?"))
// 			})

// 			It("should return an error", func() {
// 				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
// 				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
// 			})
// 		})
// 	})

// 	Context("When updating an app", func() {

// 		Context("when the app exists", func() {
// 			BeforeEach(func() {
// 				replicas := int32(3)
// 				st := &appsv1.StatefulSetList{
// 					Items: []appsv1.StatefulSet{
// 						{
// 							ObjectMeta: meta.ObjectMeta{
// 								Name: "baldur",
// 								Annotations: map[string]string{
// 									cf.ProcessGUID:          "Baldur-guid",
// 									cf.LastUpdated:          "never",
// 									eirini.RegisteredRoutes: "myroute.io",
// 								},
// 							},
// 							Spec: appsv1.StatefulSetSpec{
// 								Replicas: &replicas,
// 							},
// 						},
// 					},
// 				}

// 				statefulSetClient.ListReturns(st, nil)
// 			})

// 			Context("when update fails", func() {
// 				It("should return a meaningful message", func() {
// 					statefulSetClient.UpdateReturns(nil, errors.New("boom"))
// 					lrp := &opi.LRP{}
// 					Expect(statefulSetDesirer.Update(lrp)).To(MatchError(ContainSubstring("failed to update statefulset")))
// 				})

// 			})

// 			It("updates the statefulset", func() {
// 				lrp := &opi.LRP{
// 					TargetInstances: 5,
// 					Metadata: map[string]string{
// 						cf.LastUpdated: "now",
// 						cf.VcapAppUris: "new-route.io",
// 						"another":      "thing",
// 					},
// 				}

// 				Expect(statefulSetDesirer.Update(lrp)).To(Succeed())
// 				Expect(statefulSetClient.UpdateCallCount()).To(Equal(1))

// 				st := statefulSetClient.UpdateArgsForCall(0)
// 				Expect(st.GetAnnotations()).To(HaveKeyWithValue(cf.LastUpdated, "now"))
// 				Expect(st.GetAnnotations()).To(HaveKeyWithValue(eirini.RegisteredRoutes, "new-route.io"))
// 				Expect(st.GetAnnotations()).NotTo(HaveKey("another"))
// 				Expect(*st.Spec.Replicas).To(Equal(int32(5)))
// 			})

// 		})

// 		Context("when the app does not exist", func() {
// 			BeforeEach(func() {
// 				statefulSetClient.ListReturns(nil, errors.New("sorry"))
// 			})

// 			It("should return an error", func() {
// 				Expect(statefulSetDesirer.Update(&opi.LRP{})).
// 					To(MatchError(ContainSubstring("failed to get statefulset")))
// 			})

// 			It("should not create the app", func() {
// 				Expect(statefulSetDesirer.Update(&opi.LRP{})).
// 					To(HaveOccurred())
// 				Expect(statefulSetClient.UpdateCallCount()).To(Equal(0))
// 			})

// 		})
// 	})

// 	FContext("When listing apps", func() {
// 		It("translates all existing statefulSets to opi.LRPs", func() {
// 			expectedLRPs := []*opi.LRP{
// 				createLRP("odin", "my.example.route"),
// 				createLRP("thor", "my.example.route"),
// 				createLRP("mimir", "my.example.route"),
// 			}

// 			for _, l := range expectedLRPs {
// 				Expect(statefulSetDesirer.Desire(l)).To(Succeed())
// 			}
// 			// clean metadata and LRP because we do not return LRP
// 			// and return only subset of metadata fields
// 			for _, l := range expectedLRPs {
// 				l.Metadata = cleanupMetadata(l.Metadata)
// 				l.LRP = ""
// 			}

// 			Expect(statefulSetDesirer.List()).To(ConsistOf(expectedLRPs))
// 		})

// 		Context("no statefulSets exist", func() {
// 			It("returns an empy list of LRPs", func() {
// 				Expect(statefulSetDesirer.List()).To(BeEmpty())
// 			})
// 		})

// 		Context("fails to list the statefulsets", func() {

// 			It("should return a meaningful error", func() {
// 				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
// 					return true, nil, errors.New("boom")
// 				}
// 				client.PrependReactor("list", "statefulsets", reaction)

// 				_, err := statefulSetDesirer.List()
// 				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
// 			})

// 		})
// 	})

// 	Context("Stop an LRP", func() {

// 		BeforeEach(func() {
// 			lrp := createLRP("Baldur", "my.example.route")
// 			Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())
// 		})

// 		Context("Successful stop", func() {
// 			It("deletes the statefulSet", func() {
// 				Expect(listStatefulSets()).NotTo(BeEmpty())
// 				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())

// 				Expect(listStatefulSets()).To(BeEmpty())
// 			})
// 		})

// 		Context("when deletion of stateful set fails", func() {
// 			It("should return a meaningful error", func() {
// 				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
// 					return true, nil, errors.New("boom")
// 				}
// 				client.PrependReactor("delete", "statefulsets", reaction)
// 				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
// 					To(MatchError(ContainSubstring("failed to delete statefulset")))
// 			})
// 		})

// 		Context("when kubernetes fails to list statefulsets", func() {
// 			It("should return a meaningful error", func() {
// 				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
// 					return true, nil, errors.New("boom")
// 				}
// 				client.PrependReactor("list", "statefulsets", reaction)
// 				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).
// 					To(MatchError(ContainSubstring("failed to list statefulsets")))
// 			})

// 		})

// 		Context("when the statefulSet does not exist", func() {
// 			It("returns success", func() {
// 				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).
// 					To(Succeed())
// 			})

// 			It("logs useful information", func() {
// 				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "missing_guid", Version: "some_version"})).To(Succeed())
// 				Expect(logger).To(gbytes.Say("missing_guid"))
// 			})
// 		})
// 	})

// 	Context("Stop an LRP instance", func() {

// 		BeforeEach(func() {
// 			lrp := createLRP("Baldur", "my.example.route")
// 			Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())
// 		})

// 		It("deletes a pod instance", func() {
// 			Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
// 				To(Succeed())

// 			Expect(podClient.DeleteCallCount()).To(Equal(1))

// 			name, options := podClient.DeleteArgsForCall(0)
// 			Expect(options).To(BeNil())
// 			Expect(name).To(Equal("baldur-space-foo-random-1"))
// 		})

// 		Context("when there's an internal K8s error", func() {

// 			It("should return an error", func() {

// 				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
// 					return true, nil, errors.New("boom")
// 				}
// 				client.PrependReactor("list", "statefulsets", reaction)

// 				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
// 					To(MatchError("failed to get statefulset: boom"))
// 			})
// 		})

// 		Context("when the statefulset does not exist", func() {

// 			It("returns an error", func() {
// 				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "some", Version: "thing"}, 1)).
// 					To(MatchError("app does not exist"))
// 			})
// 		})

// 		Context("when the instance does not exist", func() {

// 			It("returns an error", func() {
// 				podClient.DeleteReturns(errors.New("boom"))
// 				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 42)).
// 					To(MatchError(ContainSubstring("failed to delete pod")))
// 			})
// 		})
// 	})

// 	Context("Get LRP instances", func() {

// 		It("should list the correct pods", func() {
// 			pods := &corev1.PodList{
// 				Items: []corev1.Pod{
// 					{ObjectMeta: meta.ObjectMeta{Name: "whatever-0"}},
// 					{ObjectMeta: meta.ObjectMeta{Name: "whatever-1"}},
// 					{ObjectMeta: meta.ObjectMeta{Name: "whatever-2"}},
// 				},
// 			}
// 			podClient.ListReturns(pods, nil)

// 			_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(podClient.ListCallCount()).To(Equal(1))
// 			Expect(podClient.ListArgsForCall(0).LabelSelector).To(Equal("guid=guid_1234,version=version_1234"))
// 		})

// 		It("should return the correct number of instances", func() {
// 			pods := &corev1.PodList{
// 				Items: []corev1.Pod{
// 					{ObjectMeta: meta.ObjectMeta{Name: "whatever-0"}},
// 					{ObjectMeta: meta.ObjectMeta{Name: "whatever-1"}},
// 					{ObjectMeta: meta.ObjectMeta{Name: "whatever-2"}},
// 				},
// 			}
// 			podClient.ListReturns(pods, nil)
// 			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(instances).To(HaveLen(3))
// 		})

// 		It("should return the correct instances information", func() {
// 			m := meta.Unix(123, 0)
// 			pods := &corev1.PodList{
// 				Items: []corev1.Pod{
// 					{
// 						ObjectMeta: meta.ObjectMeta{
// 							Name: "whatever-1",
// 						},
// 						Status: corev1.PodStatus{
// 							StartTime: &m,
// 							Phase:     corev1.PodRunning,
// 							ContainerStatuses: []corev1.ContainerStatus{
// 								{
// 									State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
// 									Ready: true,
// 								},
// 							},
// 						},
// 					},
// 				},
// 			}

// 			podClient.ListReturns(pods, nil)
// 			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(instances).To(HaveLen(1))
// 			Expect(instances[0].Index).To(Equal(1))
// 			Expect(instances[0].Since).To(Equal(int64(123000000000)))
// 			Expect(instances[0].State).To(Equal("RUNNING"))
// 			Expect(instances[0].PlacementError).To(BeEmpty())
// 		})

// 		Context("when pod list fails", func() {

// 			It("should return a meaningful error", func() {
// 				podClient.ListReturns(nil, errors.New("boom"))

// 				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
// 				Expect(err).To(MatchError(ContainSubstring("failed to list pods")))
// 			})
// 		})

// 		Context("when getting events fails", func() {

// 			It("should return a meaningful error", func() {
// 				pods := &corev1.PodList{
// 					Items: []corev1.Pod{
// 						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
// 					},
// 				}
// 				podClient.ListReturns(pods, nil)

// 				reaction := func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
// 					return true, nil, errors.New("boom")
// 				}
// 				client.PrependReactor("list", "events", reaction)

// 				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
// 				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("failed to get events for pod %s", "odin-0"))))
// 			})

// 		})

// 		Context("and time since creation is not available yet", func() {

// 			It("should return a default value", func() {
// 				pods := &corev1.PodList{
// 					Items: []corev1.Pod{
// 						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
// 					},
// 				}
// 				podClient.ListReturns(pods, nil)

// 				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
// 				Expect(err).ToNot(HaveOccurred())
// 				Expect(instances).To(HaveLen(1))
// 				Expect(instances[0].Since).To(Equal(int64(0)))
// 			})
// 		})

// 		Context("and the StatefulSet was deleted/stopped", func() {

// 			It("should return a default value", func() {
// 				event1 := &corev1.Event{
// 					Reason: "Killing",
// 					InvolvedObject: corev1.ObjectReference{
// 						Name:      "odin-0",
// 						Namespace: namespace,
// 						UID:       "odin-0-uid",
// 					},
// 				}
// 				event2 := &corev1.Event{
// 					Reason: "Killing",
// 					InvolvedObject: corev1.ObjectReference{
// 						Name:      "odin-1",
// 						Namespace: namespace,
// 						UID:       "odin-1-uid",
// 					},
// 				}

// 				event1.Name = "event1"
// 				event2.Name = "event2"

// 				_, clientErr := client.CoreV1().Events(namespace).Create(event1)
// 				Expect(clientErr).ToNot(HaveOccurred())
// 				_, clientErr = client.CoreV1().Events(namespace).Create(event2)
// 				Expect(clientErr).ToNot(HaveOccurred())

// 				pods := &corev1.PodList{
// 					Items: []corev1.Pod{
// 						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
// 					},
// 				}
// 				podClient.ListReturns(pods, nil)

// 				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
// 				Expect(err).ToNot(HaveOccurred())
// 				Expect(instances).To(HaveLen(0))
// 			})
// 		})

// 	})
// })

// func toPod(lrpName string, index int, time *meta.Time) *corev1.Pod {
// 	pod := corev1.Pod{}
// 	pod.Name = lrpName + "-" + strconv.Itoa(index)
// 	pod.UID = types.UID(pod.Name + "-uid")
// 	pod.Labels = map[string]string{
// 		"guid":    "guid_1234",
// 		"version": "version_1234",
// 	}

// 	pod.Status.StartTime = time
// 	pod.Status.Phase = corev1.PodRunning
// 	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
// 		{
// 			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
// 			Ready: true,
// 		},
// 	}
// 	return &pod
// }

// func toInstance(index int, since int64) *opi.Instance {
// 	return &opi.Instance{
// 		Index: index,
// 		Since: since,
// 		State: "RUNNING",
// 	}
// }

// const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// func randStringBytes() string {
// 	b := make([]byte, 10)
// 	for i := range b {
// 		b[i] = letterBytes[rand.Intn(len(letterBytes))]
// 	}
// 	return string(b)
// }

// func createLRP(name, routes string) *opi.LRP {
// 	lastUpdated := randStringBytes()
// 	return &opi.LRP{
// 		LRPIdentifier: opi.LRPIdentifier{
// 			GUID:    "guid_1234",
// 			Version: "version_1234",
// 		},
// 		AppName:   name,
// 		SpaceName: "space-foo",
// 		Command: []string{
// 			"/bin/sh",
// 			"-c",
// 			"while true; do echo hello; sleep 10;done",
// 		},
// 		RunningInstances: 0,
// 		MemoryMB:         1024,
// 		DiskMB:           2048,
// 		Image:            "busybox",
// 		Ports:            []int32{8888, 9999},
// 		Metadata: map[string]string{
// 			cf.ProcessGUID: name + "-guid",
// 			cf.LastUpdated: lastUpdated,
// 			cf.VcapAppUris: routes,
// 			cf.VcapAppName: name,
// 			cf.VcapAppID:   "guid_1234",
// 			cf.VcapVersion: "version_1234",
// 		},
// 		VolumeMounts: []opi.VolumeMount{
// 			{
// 				ClaimName: "some-claim",
// 				MountPath: "/some/path",
// 			},
// 		},
// 		LRP: "original request",
// 	}
// }

// func cleanupMetadata(m map[string]string) map[string]string {
// 	var fields = []string{
// 		"process_guid",
// 		"last_updated",
// 		"application_uris",
// 		"application_id",
// 		"version",
// 		"application_name",
// 	}

// 	result := map[string]string{}
// 	for _, f := range fields {
// 		result[f] = m[f]
// 	}
// 	return result
// }
