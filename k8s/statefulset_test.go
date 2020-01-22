package k8s_test

import (
	"errors"
	"fmt"
	"math/rand"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/util/utilfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace          = "testing"
	registrySecretName = "secret-name"
	rootfsVersion      = "version2"
)

var _ = Describe("Statefulset Desirer", func() {

	var (
		podClient             *k8sfakes.FakePodListerDeleter
		eventLister           *k8sfakes.FakeEventLister
		statefulSetClient     *k8sfakes.FakeStatefulSetClient
		statefulSetDesirer    opi.Desirer
		livenessProbeCreator  *k8sfakes.FakeProbeCreator
		readinessProbeCreator *k8sfakes.FakeProbeCreator
		logger                *lagertest.TestLogger
		mapper                *k8sfakes.FakeLRPMapper
	)

	BeforeEach(func() {
		podClient = new(k8sfakes.FakePodListerDeleter)
		statefulSetClient = new(k8sfakes.FakeStatefulSetClient)
		eventLister = new(k8sfakes.FakeEventLister)

		livenessProbeCreator = new(k8sfakes.FakeProbeCreator)
		readinessProbeCreator = new(k8sfakes.FakeProbeCreator)
		mapper = new(k8sfakes.FakeLRPMapper)
		hasher := new(utilfakes.FakeHasher)
		hasher.HashReturns("random", nil)
		logger = lagertest.NewTestLogger("handler-test")
		statefulSetDesirer = &StatefulSetDesirer{
			Pods:                   podClient,
			StatefulSets:           statefulSetClient,
			RegistrySecretName:     registrySecretName,
			RootfsVersion:          rootfsVersion,
			LivenessProbeCreator:   livenessProbeCreator.Spy,
			ReadinessProbeCreator:  readinessProbeCreator.Spy,
			Hasher:                 hasher,
			Logger:                 logger,
			StatefulSetToLRPMapper: mapper.Spy,
			Events:                 eventLister,
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

			It("should call the statefulset client", func() {
				Expect(statefulSetClient.CreateCallCount()).To(Equal(1))
			})

			It("should create a healthcheck probe", func() {
				Expect(livenessProbeCreator.CallCount()).To(Equal(1))
			})

			It("should create a readiness probe", func() {
				Expect(readinessProbeCreator.CallCount()).To(Equal(1))
			})

			DescribeTable("Annotations",
				func(annotationName, expectedValue string) {
					statefulSet := statefulSetClient.CreateArgsForCall(0)
					Expect(statefulSet.Annotations).To(HaveKeyWithValue(annotationName, expectedValue))
				},
				Entry("ProcessGUID", AnnotationProcessGUID, "guid_1234-version_1234"),
				Entry("AppUris", AnnotationAppUris, "my.example.route"),
				Entry("AppName", AnnotationAppName, "Baldur"),
				Entry("AppID", AnnotationAppID, "premium_app_guid_1234"),
				Entry("Version", AnnotationVersion, "version_1234"),
				Entry("OriginalRequest", AnnotationOriginalRequest, "original request"),
				Entry("RegisteredRoutes", AnnotationRegisteredRoutes, "my.example.route"),
				Entry("SpaceName", AnnotationSpaceName, "space-foo"),
				Entry("SpaceGUID", AnnotationSpaceGUID, "space-guid"),
				Entry("OrgName", AnnotationOrgName, "org-foo"),
				Entry("OrgGUID", AnnotationOrgGUID, "org-guid"),
			)

			It("should provide last updated to the statefulset annotation", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Annotations).To(HaveKeyWithValue(AnnotationLastUpdated, lrp.LastUpdated))
			})

			It("should set seccomp pod annotation", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey]).To(Equal(corev1.SeccompProfileRuntimeDefault))
			})

			It("should set name for the stateful set", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Name).To(Equal("baldur-space-foo-random"))
			})

			It("should set podManagementPolicy to parallel", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(string(statefulSet.Spec.PodManagementPolicy)).To(Equal("Parallel"))
			})

			It("should set podImagePullSecret", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
				secret := statefulSet.Spec.Template.Spec.ImagePullSecrets[0]
				Expect(secret.Name).To(Equal("secret-name"))
			})

			It("should deny privilegeEscalation", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(*statefulSet.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation).To(Equal(false))
			})

			It("should set imagePullPolicy to Always", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(string(statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy)).To(Equal("Always"))
			})

			It("should set rootfsVersion as a label", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, rootfsVersion))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, rootfsVersion))
			})

			It("should set app_guid as a label", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)

				Expect(statefulSet.Labels).To(HaveKeyWithValue(LabelAppGUID, "premium_app_guid_1234"))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(LabelAppGUID, "premium_app_guid_1234"))
			})

			It("should set process_type as a label", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Labels).To(HaveKeyWithValue(LabelProcessType, "worker"))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(LabelProcessType, "worker"))
			})

			It("should set guid as a label", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Labels).To(HaveKeyWithValue(LabelGUID, "guid_1234"))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(LabelGUID, "guid_1234"))
			})

			It("should set version as a label", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Labels).To(HaveKeyWithValue(LabelVersion, "version_1234"))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(LabelVersion, "version_1234"))
			})

			It("should set source_type as a label", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Labels).To(HaveKeyWithValue(LabelSourceType, "APP"))
				Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(LabelSourceType, "APP"))
			})

			It("should set guid as a label selector", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Selector.MatchLabels).To(HaveKeyWithValue(LabelGUID, "guid_1234"))
			})

			It("should set version as a label selector", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Selector.MatchLabels).To(HaveKeyWithValue(LabelVersion, "version_1234"))
			})

			It("should set source_type as a label selector", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Selector.MatchLabels).To(HaveKeyWithValue(LabelSourceType, "APP"))
			})

			It("should set disk limit", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)

				expectedLimit := resource.NewScaledQuantity(2048, resource.Mega)
				actualLimit := statefulSet.Spec.Template.Spec.Containers[0].Resources.Limits.StorageEphemeral()
				Expect(actualLimit).To(Equal(expectedLimit))
			})

			It("should set user defined annotations", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Annotations["prometheus.io/scrape"]).To(Equal("secret-value"))
			})

			It("should run it with non-root user", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(Equal(true)))

			})

			It("should run it as vcap user with numerical ID 2000", func() {
				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Spec.SecurityContext.RunAsUser).To(PointTo(Equal(int64(2000))))
			})
		})

		Context("When the app name contains unsupported characters", func() {
			It("should use the guid as a name", func() {
				lrp := createLRP("Балдър", "my.example.route")
				Expect(statefulSetDesirer.Desire(lrp)).To(Succeed())

				statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Name).To(Equal("guid_1234-random"))
			})
		})
	})

	Context("When getting an app", func() {

		BeforeEach(func() {
			mapper.Returns(&opi.LRP{AppName: "baldur-app"})
		})

		It("should use mapper to get LRP", func() {
			st := &appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{
					{
						ObjectMeta: meta.ObjectMeta{
							Name: "baldur",
						},
					},
				},
			}

			statefulSetClient.ListReturns(st, nil)
			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
			Expect(mapper.CallCount()).To(Equal(1))
			Expect(lrp.AppName).To(Equal("baldur-app"))
		})

		Context("when the app does not exist", func() {

			BeforeEach(func() {
				statefulSetClient.ListReturns(&appsv1.StatefulSetList{}, nil)
			})

			It("should return an error", func() {
				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("statefulset not found")))
			})
		})

		Context("when statefulsets cannot be listed", func() {

			BeforeEach(func() {
				statefulSetClient.ListReturns(nil, errors.New("who is this?"))
			})

			It("should return an error", func() {
				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})
	})

	Context("When updating an app", func() {

		Context("when the app exists", func() {
			BeforeEach(func() {
				replicas := int32(3)
				st := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{
						{
							ObjectMeta: meta.ObjectMeta{
								Name: "baldur",
								Annotations: map[string]string{
									AnnotationProcessGUID:      "Baldur-guid",
									AnnotationLastUpdated:      "never",
									AnnotationRegisteredRoutes: "myroute.io",
								},
							},
							Spec: appsv1.StatefulSetSpec{
								Replicas: &replicas,
							},
						},
					},
				}

				statefulSetClient.ListReturns(st, nil)
			})

			Context("when update fails", func() {
				It("should return a meaningful message", func() {
					statefulSetClient.UpdateReturns(nil, errors.New("boom"))
					lrp := &opi.LRP{}
					Expect(statefulSetDesirer.Update(lrp)).To(MatchError(ContainSubstring("failed to update statefulset")))
				})

			})

			It("updates the statefulset", func() {
				lrp := &opi.LRP{
					TargetInstances: 5,
					LastUpdated:     "now",
					AppURIs:         "new-route.io",
				}

				Expect(statefulSetDesirer.Update(lrp)).To(Succeed())
				Expect(statefulSetClient.UpdateCallCount()).To(Equal(1))

				st := statefulSetClient.UpdateArgsForCall(0)
				Expect(st.GetAnnotations()).To(HaveKeyWithValue(AnnotationLastUpdated, "now"))
				Expect(st.GetAnnotations()).To(HaveKeyWithValue(AnnotationRegisteredRoutes, "new-route.io"))
				Expect(st.GetAnnotations()).NotTo(HaveKey("another"))
				Expect(*st.Spec.Replicas).To(Equal(int32(5)))
			})

		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				statefulSetClient.ListReturns(nil, errors.New("sorry"))
			})

			It("should return an error", func() {
				Expect(statefulSetDesirer.Update(&opi.LRP{})).
					To(MatchError(ContainSubstring("failed to get statefulset")))
			})

			It("should not create the app", func() {
				Expect(statefulSetDesirer.Update(&opi.LRP{})).
					To(HaveOccurred())
				Expect(statefulSetClient.UpdateCallCount()).To(Equal(0))
			})

		})
	})

	Context("When listing apps", func() {
		It("translates all existing statefulSets to opi.LRPs", func() {
			st := &appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{
					{
						ObjectMeta: meta.ObjectMeta{
							Name: "odin",
						},
					},
					{
						ObjectMeta: meta.ObjectMeta{
							Name: "thor",
						},
					},
					{
						ObjectMeta: meta.ObjectMeta{
							Name: "baldur",
						},
					},
				},
			}

			statefulSetClient.ListReturns(st, nil)

			Expect(statefulSetDesirer.List()).To(HaveLen(3))
			Expect(mapper.CallCount()).To(Equal(3))
		})

		Context("no statefulSets exist", func() {
			It("returns an empy list of LRPs", func() {
				statefulSetClient.ListReturns(&appsv1.StatefulSetList{}, nil)
				Expect(statefulSetDesirer.List()).To(BeEmpty())
				Expect(mapper.CallCount()).To(Equal(0))
			})
		})

		Context("fails to list the statefulsets", func() {

			It("should return a meaningful error", func() {
				statefulSetClient.ListReturns(nil, errors.New("who is this?"))
				_, err := statefulSetDesirer.List()
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})

		})
	})

	Context("Stop an LRP", func() {
		Context("Successful stop", func() {
			It("deletes the statefulSet", func() {
				st := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{
						{
							ObjectMeta: meta.ObjectMeta{
								Name: "baldur",
							},
						},
					},
				}

				statefulSetClient.ListReturns(st, nil)
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				Expect(statefulSetClient.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("when deletion of stateful set fails", func() {
			It("should return a meaningful error", func() {
				st := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{
						{
							ObjectMeta: meta.ObjectMeta{
								Name: "baldur",
							},
						},
					},
				}

				statefulSetClient.ListReturns(st, nil)
				statefulSetClient.DeleteReturns(errors.New("boom"))
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to delete statefulset")))
			})
		})

		Context("when kubernetes fails to list statefulsets", func() {
			It("should return a meaningful error", func() {
				statefulSetClient.ListReturns(nil, errors.New("who is this?"))
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})

		})

		Context("when the statefulSet does not exist", func() {
			BeforeEach(func() {
				statefulSetClient.ListReturns(&appsv1.StatefulSetList{}, nil)
			})

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
		It("deletes a pod instance", func() {
			st := &appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{
					{
						ObjectMeta: meta.ObjectMeta{
							Name: "baldur-space-foo-random",
						},
					},
				},
			}

			statefulSetClient.ListReturns(st, nil)

			Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
				To(Succeed())

			Expect(podClient.DeleteCallCount()).To(Equal(1))

			name, options := podClient.DeleteArgsForCall(0)
			Expect(options).To(BeNil())
			Expect(name).To(Equal("baldur-space-foo-random-1"))
		})

		Context("when there's an internal K8s error", func() {
			It("should return an error", func() {
				statefulSetClient.ListReturns(nil, errors.New("boom"))
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(MatchError("failed to get statefulset: boom"))
			})
		})

		Context("when the statefulset does not exist", func() {

			It("returns an error", func() {
				statefulSetClient.ListReturns(&appsv1.StatefulSetList{}, nil)
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "some", Version: "thing"}, 1)).
					To(MatchError("app does not exist"))
			})
		})

		Context("when the instance does not exist", func() {

			It("returns an error", func() {
				st := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{
						{
							ObjectMeta: meta.ObjectMeta{
								Name: "baldur",
							},
						},
					},
				}

				statefulSetClient.ListReturns(st, nil)
				podClient.DeleteReturns(errors.New("boom"))
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 42)).
					To(MatchError(ContainSubstring("failed to delete pod")))
			})
		})
	})

	Context("Get LRP instances", func() {

		It("should list the correct pods", func() {
			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: meta.ObjectMeta{Name: "whatever-0"}},
					{ObjectMeta: meta.ObjectMeta{Name: "whatever-1"}},
					{ObjectMeta: meta.ObjectMeta{Name: "whatever-2"}},
				},
			}
			podClient.ListReturns(pods, nil)
			eventLister.ListReturns(&corev1.EventList{}, nil)

			_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

			Expect(err).ToNot(HaveOccurred())
			Expect(podClient.ListCallCount()).To(Equal(1))
			Expect(podClient.ListArgsForCall(0).LabelSelector).To(Equal("cloudfoundry.org/guid=guid_1234,cloudfoundry.org/version=version_1234"))
		})

		It("should return the correct number of instances", func() {
			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: meta.ObjectMeta{Name: "whatever-0"}},
					{ObjectMeta: meta.ObjectMeta{Name: "whatever-1"}},
					{ObjectMeta: meta.ObjectMeta{Name: "whatever-2"}},
				},
			}
			podClient.ListReturns(pods, nil)
			eventLister.ListReturns(&corev1.EventList{}, nil)
			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(3))
		})

		It("should return the correct instances information", func() {
			m := meta.Unix(123, 0)
			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: meta.ObjectMeta{
							Name: "whatever-1",
						},
						Status: corev1.PodStatus{
							StartTime: &m,
							Phase:     corev1.PodRunning,
							ContainerStatuses: []corev1.ContainerStatus{
								{
									State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
									Ready: true,
								},
							},
						},
					},
				},
			}

			podClient.ListReturns(pods, nil)
			eventLister.ListReturns(&corev1.EventList{}, nil)
			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(1))
			Expect(instances[0].Index).To(Equal(1))
			Expect(instances[0].Since).To(Equal(int64(123000000000)))
			Expect(instances[0].State).To(Equal("RUNNING"))
			Expect(instances[0].PlacementError).To(BeEmpty())
		})

		Context("when pod list fails", func() {

			It("should return a meaningful error", func() {
				podClient.ListReturns(nil, errors.New("boom"))

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
				Expect(err).To(MatchError(ContainSubstring("failed to list pods")))
			})
		})

		Context("when getting events fails", func() {

			It("should return a meaningful error", func() {
				pods := &corev1.PodList{
					Items: []corev1.Pod{
						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
					},
				}
				podClient.ListReturns(pods, nil)

				eventLister.ListReturns(nil, errors.New("I am error"))

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("failed to get events for pod %s", "odin-0"))))
			})

		})

		Context("and time since creation is not available yet", func() {

			It("should return a default value", func() {
				pods := &corev1.PodList{
					Items: []corev1.Pod{
						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
					},
				}
				podClient.ListReturns(pods, nil)
				eventLister.ListReturns(&corev1.EventList{}, nil)

				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(1))
				Expect(instances[0].Since).To(Equal(int64(0)))
			})
		})

		Context("and pods needs too much resources", func() {
			BeforeEach(func() {
				pods := &corev1.PodList{
					Items: []corev1.Pod{
						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
					},
				}
				podClient.ListReturns(pods, nil)
			})

			Context("and the cluster has autoscaler", func() {
				BeforeEach(func() {
					eventLister.ListReturns(&corev1.EventList{
						Items: []corev1.Event{
							{
								Reason:  "NotTriggerScaleUp",
								Message: "pod didn't trigger scale-up (it wouldn't fit if a new node is added): 1 Insufficient memory",
							},
						},
					}, nil)
				})

				It("returns insufficient memory response", func() {
					instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
					Expect(err).ToNot(HaveOccurred())
					Expect(instances).To(HaveLen(1))
					Expect(instances[0].PlacementError).To(Equal(opi.InsufficientMemoryError))
				})
			})

			Context("and the cluster does not have autoscaler", func() {
				BeforeEach(func() {
					eventLister.ListReturns(&corev1.EventList{
						Items: []corev1.Event{
							{
								Reason:  "FailedScheduling",
								Message: "0/3 nodes are available: 3 Insufficient memory.",
							},
						},
					}, nil)
				})

				It("returns insufficient memory response", func() {
					instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
					Expect(err).ToNot(HaveOccurred())
					Expect(instances).To(HaveLen(1))
					Expect(instances[0].PlacementError).To(Equal(opi.InsufficientMemoryError))
				})
			})
		})

		Context("and the StatefulSet was deleted/stopped", func() {

			It("should return a default value", func() {
				event1 := corev1.Event{
					Reason: "Killing",
					InvolvedObject: corev1.ObjectReference{
						Name:      "odin-0",
						Namespace: namespace,
						UID:       "odin-0-uid",
					},
				}
				event2 := corev1.Event{
					Reason: "Killing",
					InvolvedObject: corev1.ObjectReference{
						Name:      "odin-1",
						Namespace: namespace,
						UID:       "odin-1-uid",
					},
				}
				eventLister.ListReturns(&corev1.EventList{
					Items: []corev1.Event{
						event1,
						event2,
					},
				}, nil)

				pods := &corev1.PodList{
					Items: []corev1.Pod{
						{ObjectMeta: meta.ObjectMeta{Name: "odin-0"}},
					},
				}
				podClient.ListReturns(pods, nil)

				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(0))
			})
		})

	})
})

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
		ProcessType: "worker",
		AppName:     name,
		AppGUID:     "premium_app_guid_1234",
		SpaceName:   "space-foo",
		SpaceGUID:   "space-guid",
		OrgName:     "org-foo",
		OrgGUID:     "org-guid",
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
		LastUpdated:      lastUpdated,
		AppURIs:          routes,
		VolumeMounts: []opi.VolumeMount{
			{
				ClaimName: "some-claim",
				MountPath: "/some/path",
			},
		},
		LRP: "original request",
		UserDefinedAnnotations: map[string]string{
			"prometheus.io/scrape": "secret-value",
		},
	}
}
