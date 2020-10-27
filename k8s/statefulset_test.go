package k8s_test

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	namespace          = "testing"
	registrySecretName = "secret-name"
)

var _ = Describe("Statefulset Desirer", func() {
	var (
		podsClient            *k8sfakes.FakePodClient
		eventsClient          *k8sfakes.FakeEventsClient
		secretsClient         *k8sfakes.FakeSecretsCreatorDeleter
		statefulSetClient     *k8sfakes.FakeStatefulSetClient
		statefulSetDesirer    *k8s.StatefulSetDesirer
		livenessProbeCreator  *k8sfakes.FakeProbeCreator
		readinessProbeCreator *k8sfakes.FakeProbeCreator
		logger                *lagertest.TestLogger
		mapper                *k8sfakes.FakeLRPMapper
		pdbClient             *k8sfakes.FakePodDisruptionBudgetClient
	)

	BeforeEach(func() {
		podsClient = new(k8sfakes.FakePodClient)
		statefulSetClient = new(k8sfakes.FakeStatefulSetClient)
		secretsClient = new(k8sfakes.FakeSecretsCreatorDeleter)
		eventsClient = new(k8sfakes.FakeEventsClient)

		livenessProbeCreator = new(k8sfakes.FakeProbeCreator)
		readinessProbeCreator = new(k8sfakes.FakeProbeCreator)
		mapper = new(k8sfakes.FakeLRPMapper)
		pdbClient = new(k8sfakes.FakePodDisruptionBudgetClient)

		logger = lagertest.NewTestLogger("handler-test")
		statefulSetDesirer = &k8s.StatefulSetDesirer{
			Pods:                      podsClient,
			Secrets:                   secretsClient,
			StatefulSets:              statefulSetClient,
			PodDisruptionBudgets:      pdbClient,
			RegistrySecretName:        registrySecretName,
			LivenessProbeCreator:      livenessProbeCreator.Spy,
			ReadinessProbeCreator:     readinessProbeCreator.Spy,
			Logger:                    logger,
			StatefulSetToLRPMapper:    mapper.Spy,
			EventsClient:              eventsClient,
			ApplicationServiceAccount: "eirini",
		}
	})

	Describe("Desire", func() {
		var (
			lrp                        *opi.LRP
			desireErr                  error
			desireOptOne, desireOptTwo *k8sfakes.FakeDesireOption
		)

		BeforeEach(func() {
			lrp = createLRP("Baldur", []opi.Route{{Hostname: "my.example.route", Port: 1000}})
			livenessProbeCreator.Returns(&corev1.Probe{})
			readinessProbeCreator.Returns(&corev1.Probe{})
			desireOptOne = new(k8sfakes.FakeDesireOption)
			desireOptTwo = new(k8sfakes.FakeDesireOption)
		})

		JustBeforeEach(func() {
			desireErr = statefulSetDesirer.Desire("the-namespace", lrp, desireOptOne.Spy, desireOptTwo.Spy)
		})

		It("should succeed", func() {
			Expect(desireErr).NotTo(HaveOccurred())
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

		It("should invoke the opts with the StatefulSet", func() {
			Expect(desireOptOne.CallCount()).To(Equal(1))
			Expect(desireOptTwo.CallCount()).To(Equal(1))

			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(desireOptOne.ArgsForCall(0)).To(Equal(statefulSet))
			Expect(desireOptTwo.ArgsForCall(0)).To(Equal(statefulSet))
		})

		DescribeTable("Statefulset Annotations",
			func(annotationName, expectedValue string) {
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Annotations).To(HaveKeyWithValue(annotationName, expectedValue))
			},
			Entry("ProcessGUID", k8s.AnnotationProcessGUID, "guid_1234-version_1234"),
			Entry("AppName", k8s.AnnotationAppName, "Baldur"),
			Entry("AppID", k8s.AnnotationAppID, "premium_app_guid_1234"),
			Entry("Version", k8s.AnnotationVersion, "version_1234"),
			Entry("OriginalRequest", k8s.AnnotationOriginalRequest, "original request"),
			Entry("RegisteredRoutes", k8s.AnnotationRegisteredRoutes, `[{"hostname":"my.example.route","port":1000}]`),
			Entry("SpaceName", k8s.AnnotationSpaceName, "space-foo"),
			Entry("SpaceGUID", k8s.AnnotationSpaceGUID, "space-guid"),
			Entry("OrgName", k8s.AnnotationOrgName, "org-foo"),
			Entry("OrgGUID", k8s.AnnotationOrgGUID, "org-guid"),
		)

		DescribeTable("Statefulset Template Annotations",
			func(annotationName, expectedValue string) {
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Annotations).To(HaveKeyWithValue(annotationName, expectedValue))
			},
			Entry("ProcessGUID", k8s.AnnotationProcessGUID, "guid_1234-version_1234"),
			Entry("AppName", k8s.AnnotationAppName, "Baldur"),
			Entry("AppID", k8s.AnnotationAppID, "premium_app_guid_1234"),
			Entry("Version", k8s.AnnotationVersion, "version_1234"),
			Entry("OriginalRequest", k8s.AnnotationOriginalRequest, "original request"),
			Entry("RegisteredRoutes", k8s.AnnotationRegisteredRoutes, `[{"hostname":"my.example.route","port":1000}]`),
			Entry("SpaceName", k8s.AnnotationSpaceName, "space-foo"),
			Entry("SpaceGUID", k8s.AnnotationSpaceGUID, "space-guid"),
			Entry("OrgName", k8s.AnnotationOrgName, "org-foo"),
			Entry("OrgGUID", k8s.AnnotationOrgGUID, "org-guid"),
		)

		It("should provide last updated to the statefulset annotation", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Annotations).To(HaveKeyWithValue(k8s.AnnotationLastUpdated, lrp.LastUpdated))
		})

		It("should set seccomp pod annotation", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Annotations[corev1.SeccompPodAnnotationKey]).To(Equal(corev1.SeccompProfileRuntimeDefault))
		})

		It("should set name for the stateful set", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Name).To(Equal("baldur-space-foo-34f869d015"))
		})

		It("should set namespace for the stateful set", func() {
			namespace, _ := statefulSetClient.CreateArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
		})

		It("should set podManagementPolicy to parallel", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(string(statefulSet.Spec.PodManagementPolicy)).To(Equal("Parallel"))
		})

		It("should set podImagePullSecret", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
			secret := statefulSet.Spec.Template.Spec.ImagePullSecrets[0]
			Expect(secret.Name).To(Equal("secret-name"))
		})

		It("should deny privilegeEscalation", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(*statefulSet.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation).To(Equal(false))
		})

		It("should not automount service account token", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			f := false
			Expect(statefulSet.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(&f))
		})

		It("should set imagePullPolicy to Always", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(string(statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy)).To(Equal("Always"))
		})

		It("should set app_guid as a label", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)

			Expect(statefulSet.Labels).To(HaveKeyWithValue(k8s.LabelAppGUID, "premium_app_guid_1234"))
			Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(k8s.LabelAppGUID, "premium_app_guid_1234"))
		})

		It("should set process_type as a label", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Labels).To(HaveKeyWithValue(k8s.LabelProcessType, "worker"))
			Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(k8s.LabelProcessType, "worker"))
		})

		It("should set guid as a label", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Labels).To(HaveKeyWithValue(k8s.LabelGUID, "guid_1234"))
			Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(k8s.LabelGUID, "guid_1234"))
		})

		It("should set version as a label", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Labels).To(HaveKeyWithValue(k8s.LabelVersion, "version_1234"))
			Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(k8s.LabelVersion, "version_1234"))
		})

		It("should set source_type as a label", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Labels).To(HaveKeyWithValue(k8s.LabelSourceType, "APP"))
			Expect(statefulSet.Spec.Template.Labels).To(HaveKeyWithValue(k8s.LabelSourceType, "APP"))
		})

		It("should set guid as a label selector", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Selector.MatchLabels).To(HaveKeyWithValue(k8s.LabelGUID, "guid_1234"))
		})

		It("should set version as a label selector", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Selector.MatchLabels).To(HaveKeyWithValue(k8s.LabelVersion, "version_1234"))
		})

		It("should set source_type as a label selector", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Selector.MatchLabels).To(HaveKeyWithValue(k8s.LabelSourceType, "APP"))
		})

		It("should set disk limit", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)

			expectedLimit := resource.NewScaledQuantity(2048, resource.Mega)
			actualLimit := statefulSet.Spec.Template.Spec.Containers[0].Resources.Limits.StorageEphemeral()
			Expect(actualLimit).To(Equal(expectedLimit))
		})

		It("should set user defined annotations", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Annotations["prometheus.io/scrape"]).To(Equal("secret-value"))
		})

		It("should run it with non-root user", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(Equal(true)))
		})

		It("should run it as vcap user with numerical ID 2000", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Spec.SecurityContext.RunAsUser).To(PointTo(Equal(int64(2000))))
		})

		It("should not create a pod disruption budget", func() {
			Expect(pdbClient.CreateCallCount()).To(BeZero())
		})

		It("should set soft inter-pod anti-affinity", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			podAntiAffinity := statefulSet.Spec.Template.Spec.Affinity.PodAntiAffinity
			Expect(podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(BeEmpty())
			Expect(podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(1))

			weightedTerm := podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0]
			Expect(weightedTerm.Weight).To(Equal(int32(100)))
			Expect(weightedTerm.PodAffinityTerm.TopologyKey).To(Equal("kubernetes.io/hostname"))
			Expect(weightedTerm.PodAffinityTerm.LabelSelector.MatchExpressions).To(ConsistOf(
				metav1.LabelSelectorRequirement{
					Key:      k8s.LabelGUID,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"guid_1234"},
				},
				metav1.LabelSelectorRequirement{
					Key:      k8s.LabelVersion,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"version_1234"},
				},
				metav1.LabelSelectorRequirement{
					Key:      k8s.LabelSourceType,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"APP"},
				},
			))
		})

		It("should set application service account", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Spec.ServiceAccountName).To(Equal("eirini"))
		})

		It("should set the container environment variables", func() {
			_, statefulSet := statefulSetClient.CreateArgsForCall(0)
			Expect(statefulSet.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := statefulSet.Spec.Template.Spec.Containers[0]
			Expect(container.Env).To(ContainElements(
				corev1.EnvVar{Name: eirini.EnvPodName, ValueFrom: expectedValFrom("metadata.name")},
				corev1.EnvVar{Name: eirini.EnvCFInstanceGUID, ValueFrom: expectedValFrom("metadata.uid")},
				corev1.EnvVar{Name: eirini.EnvCFInstanceInternalIP, ValueFrom: expectedValFrom("status.podIP")},
				corev1.EnvVar{Name: eirini.EnvCFInstanceIP, ValueFrom: expectedValFrom("status.hostIP")},
			))
		})

		When("automounting service account token is allowed", func() {
			BeforeEach(func() {
				statefulSetDesirer.AllowAutomountServiceAccountToken = true
			})

			It("does not set automountServiceAccountToken", func() {
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Spec.AutomountServiceAccountToken).To(BeNil())
			})
		})

		When("application should run as root", func() {
			BeforeEach(func() {
				lrp.RunsAsRoot = true
			})

			It("does not set privileged context", func() {
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Spec.SecurityContext).To(BeNil())
			})
		})

		When("the app name contains unsupported characters", func() {
			BeforeEach(func() {
				lrp = createLRP("Балдър", []opi.Route{{Hostname: "my.example.route", Port: 10000}})
			})

			It("should use the guid as a name", func() {
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Name).To(Equal("guid_1234-34f869d015"))
			})
		})

		When("the app has at least 2 instances", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 2
			})

			It("should create a pod disruption budget for it", func() {
				Expect(pdbClient.CreateCallCount()).To(Equal(1))

				pdbNamespace, pdb := pdbClient.CreateArgsForCall(0)
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(pdbNamespace).To(Equal("the-namespace"))
				Expect(pdb.Name).To(Equal(statefulSet.Name))
				Expect(pdb.Spec.MinAvailable).To(PointTo(Equal(intstr.FromInt(1))))
				Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(k8s.LabelGUID, lrp.GUID))
				Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(k8s.LabelVersion, lrp.Version))
				Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(k8s.LabelSourceType, "APP"))
			})

			When("pod disruption budget creation fails", func() {
				BeforeEach(func() {
					pdbClient.CreateReturns(nil, errors.New("boom"))
				})

				It("should propagate the error", func() {
					Expect(desireErr).To(MatchError(ContainSubstring("boom")))
				})
			})

			When("the statefulset already exists", func() {
				BeforeEach(func() {
					statefulSetClient.CreateReturns(nil, k8serrors.NewAlreadyExists(schema.GroupResource{}, "potato"))
				})

				It("does not fail", func() {
					Expect(desireErr).NotTo(HaveOccurred())
				})
			})

			When("creating the statefulset fails", func() {
				BeforeEach(func() {
					statefulSetClient.CreateReturns(nil, errors.New("potato"))
				})

				It("propagates the error", func() {
					Expect(desireErr).To(MatchError(ContainSubstring("potato")))
				})
			})
		})

		When("the app references a private docker image", func() {
			BeforeEach(func() {
				lrp.PrivateRegistry = &opi.PrivateRegistry{
					Server:   "host",
					Username: "user",
					Password: "password",
				}
			})

			It("should create a private repo secret containing the private repo credentials", func() {
				Expect(secretsClient.CreateCallCount()).To(Equal(1))
				secretNamespace, actualSecret := secretsClient.CreateArgsForCall(0)
				Expect(secretNamespace).To(Equal("the-namespace"))
				Expect(actualSecret.Name).To(Equal("baldur-space-foo-34f869d015-registry-credentials"))
				Expect(actualSecret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
				Expect(actualSecret.StringData).To(
					HaveKeyWithValue(
						".dockerconfigjson",
						fmt.Sprintf(
							`{"auths":{"host":{"username":"user","password":"password","auth":"%s"}}}`,
							base64.StdEncoding.EncodeToString([]byte("user:password")),
						),
					),
				)
			})

			It("should add the private repo secret to podImagePullSecret", func() {
				Expect(statefulSetClient.CreateCallCount()).To(Equal(1))
				_, statefulSet := statefulSetClient.CreateArgsForCall(0)
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(2))
				secret := statefulSet.Spec.Template.Spec.ImagePullSecrets[1]
				Expect(secret.Name).To(Equal("baldur-space-foo-34f869d015-registry-credentials"))
			})
		})
	})

	Describe("Get", func() {
		BeforeEach(func() {
			mapper.Returns(&opi.LRP{AppName: "baldur-app"}, nil)
		})

		It("should use mapper to get LRP", func() {
			st := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "baldur",
				},
			}

			statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{st}, nil)
			lrp, _ := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
			Expect(mapper.CallCount()).To(Equal(1))
			Expect(lrp.AppName).To(Equal("baldur-app"))
		})

		When("the app does not exist", func() {
			BeforeEach(func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
			})

			It("should return an error", func() {
				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})

		When("statefulsets cannot be listed", func() {
			BeforeEach(func() {
				statefulSetClient.GetByLRPIdentifierReturns(nil, errors.New("who is this?"))
			})

			It("should return an error", func() {
				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("there are 2 lrps with the same identifier", func() {
			BeforeEach(func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}, {}}, nil)
			})

			It("should return an error", func() {
				_, err := statefulSetDesirer.Get(opi.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("multiple statefulsets found for LRP identifier")))
			})
		})
	})

	Describe("Update", func() {
		var (
			updatedLRP *opi.LRP
			err        error
		)

		BeforeEach(func() {
			updatedLRP = &opi.LRP{
				LRPIdentifier: opi.LRPIdentifier{
					GUID:    "guid_1234",
					Version: "version_1234",
				},
				AppName:         "baldur",
				SpaceName:       "space-foo",
				TargetInstances: 5,
				LastUpdated:     "now",
				AppURIs:         []opi.Route{{Hostname: "new-route.io", Port: 6666}},
				Image:           "new/image",
			}

			replicas := int32(3)

			st := []appsv1.StatefulSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baldur",
						Namespace: "the-namespace",
						Annotations: map[string]string{
							k8s.AnnotationProcessGUID:      "Baldur-guid",
							k8s.AnnotationLastUpdated:      "never",
							k8s.AnnotationRegisteredRoutes: `[{"hostname":"myroute.io","port":1000}]`,
						},
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: &replicas,
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "another-container", Image: "another/image"},
									{Name: k8s.OPIContainerName, Image: "old/image"},
								},
							},
						},
					},
				},
			}

			statefulSetClient.GetByLRPIdentifierReturns(st, nil)
		})

		JustBeforeEach(func() {
			err = statefulSetDesirer.Update(updatedLRP)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the statefulset", func() {
			Expect(statefulSetClient.UpdateCallCount()).To(Equal(1))

			namespace, st := statefulSetClient.UpdateArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(st.GetAnnotations()).To(HaveKeyWithValue(k8s.AnnotationLastUpdated, "now"))
			Expect(st.GetAnnotations()).To(HaveKeyWithValue(k8s.AnnotationRegisteredRoutes, `[{"hostname":"new-route.io","port":6666}]`))
			Expect(st.GetAnnotations()).NotTo(HaveKey("another"))
			Expect(*st.Spec.Replicas).To(Equal(int32(5)))
			Expect(st.Spec.Template.Spec.Containers[0].Image).To(Equal("another/image"))
			Expect(st.Spec.Template.Spec.Containers[1].Image).To(Equal("new/image"))
		})

		When("the image is missing", func() {
			BeforeEach(func() {
				updatedLRP.Image = ""
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't reset the image", func() {
				Expect(statefulSetClient.UpdateCallCount()).To(Equal(1))

				_, st := statefulSetClient.UpdateArgsForCall(0)
				Expect(st.Spec.Template.Spec.Containers[1].Image).To(Equal("old/image"))
			})
		})

		When("lrp is scaled down to 1 instance", func() {
			BeforeEach(func() {
				updatedLRP.TargetInstances = 1
			})

			It("should delete the pod disruption budget for the lrp", func() {
				Expect(pdbClient.DeleteCallCount()).To(Equal(1))
				pdbNamespace, pdbName := pdbClient.DeleteArgsForCall(0)
				Expect(pdbNamespace).To(Equal("the-namespace"))
				Expect(pdbName).To(Equal("baldur"))
			})

			When("the pod disruption budget does not exist", func() {
				BeforeEach(func() {
					pdbClient.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{
						Group:    "policy/v1beta1",
						Resource: "PodDisruptionBudget",
					}, "baldur"))
				})

				It("should ignore the error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("the pod disruption budget deletion errors", func() {
				BeforeEach(func() {
					pdbClient.DeleteReturns(errors.New("pow"))
				})

				It("should propagate the error", func() {
					Expect(err).To(MatchError(ContainSubstring("pow")))
				})
			})
		})

		When("lrp is scaled up to more than 1 instance", func() {
			BeforeEach(func() {
				updatedLRP.TargetInstances = 2
			})

			It("should create a pod disruption budget for the lrp in the same namespace", func() {
				Expect(pdbClient.CreateCallCount()).To(Equal(1))
				namespace, pdb := pdbClient.CreateArgsForCall(0)

				Expect(pdb.Name).To(Equal("baldur"))
				Expect(namespace).To(Equal("the-namespace"))
			})

			When("the pod disruption budget already exists", func() {
				BeforeEach(func() {
					pdbClient.CreateReturns(nil, k8serrors.NewAlreadyExists(schema.GroupResource{
						Group:    "policy/v1beta1",
						Resource: "PodDisruptionBudget",
					}, "baldur"))
				})

				It("should ignore the error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("the pod disruption budget creation errors", func() {
				BeforeEach(func() {
					pdbClient.CreateReturns(nil, errors.New("boom"))
				})

				It("should propagate the error", func() {
					Expect(err).To(MatchError(ContainSubstring("boom")))
				})
			})
		})

		When("update fails", func() {
			BeforeEach(func() {
				statefulSetClient.UpdateReturns(nil, errors.New("boom"))
			})

			It("should return a meaningful message", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to update statefulset")))
			})
		})

		When("update fails because of a conflict", func() {
			BeforeEach(func() {
				statefulSetClient.UpdateReturnsOnCall(0, nil, k8serrors.NewConflict(schema.GroupResource{}, "foo", errors.New("boom")))
				statefulSetClient.UpdateReturnsOnCall(1, &appsv1.StatefulSet{}, nil)
			})

			It("should retry", func() {
				Expect(statefulSetClient.UpdateCallCount()).To(Equal(2))
			})
		})

		When("the app does not exist", func() {
			BeforeEach(func() {
				statefulSetClient.GetByLRPIdentifierReturns(nil, errors.New("sorry"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})

			It("should not create the app", func() {
				Expect(statefulSetClient.UpdateCallCount()).To(Equal(0))
			})
		})
	})

	Context("When listing apps", func() {
		It("translates all existing statefulSets to opi.LRPs", func() {
			st := []appsv1.StatefulSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "odin",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "thor",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "baldur",
					},
				},
			}

			statefulSetClient.GetBySourceTypeReturns(st, nil)

			Expect(statefulSetDesirer.List()).To(HaveLen(3))
			Expect(mapper.CallCount()).To(Equal(3))
		})

		It("lists all statefulSets with APP source_type", func() {
			statefulSetClient.GetBySourceTypeReturns([]appsv1.StatefulSet{}, nil)
			_, err := statefulSetDesirer.List()
			Expect(err).NotTo(HaveOccurred())

			Expect(statefulSetClient.GetBySourceTypeCallCount()).To(Equal(1))

			sourceType := statefulSetClient.GetBySourceTypeArgsForCall(0)
			Expect(sourceType).To(Equal("APP"))
		})

		When("no statefulSets exist", func() {
			It("returns an empy list of LRPs", func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
				Expect(statefulSetDesirer.List()).To(BeEmpty())
				Expect(mapper.CallCount()).To(Equal(0))
			})
		})

		When("listing statefulsets fails", func() {
			It("should return a meaningful error", func() {
				statefulSetClient.GetBySourceTypeReturns(nil, errors.New("who is this?"))
				_, err := statefulSetDesirer.List()
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})
	})

	Describe("Stop", func() {
		var statefulSets []appsv1.StatefulSet

		BeforeEach(func() {
			statefulSets = []appsv1.StatefulSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baldur",
						Namespace: "the-namespace",
					},
				},
			}
			statefulSetClient.GetByLRPIdentifierReturns(statefulSets, nil)
			pdbClient.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{
				Group:    "policy/v1beta1",
				Resource: "PodDisruptionBudet",
			},
				"foo"))
		})

		It("deletes the statefulSet", func() {
			Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
			Expect(statefulSetClient.DeleteCallCount()).To(Equal(1))
			namespace, name := statefulSetClient.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(name).To(Equal("baldur"))
		})

		It("should delete any corresponding pod disruption budgets", func() {
			Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
			Expect(pdbClient.DeleteCallCount()).To(Equal(1))
			namespace, pdbName := pdbClient.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(pdbName).To(Equal("baldur"))
		})

		When("the stateful set runs an image from a private registry", func() {
			BeforeEach(func() {
				statefulSets[0].Spec = appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "baldur-registry-credentials"},
							},
						},
					},
				}
			})

			It("deletes the secret holding the creds of the private registry", func() {
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				Expect(secretsClient.DeleteCallCount()).To(Equal(1))
				secretNs, secretName := secretsClient.DeleteArgsForCall(0)
				Expect(secretName).To(Equal("baldur-registry-credentials"))
				Expect(secretNs).To(Equal("the-namespace"))
			})

			When("deleting the private registry secret fails", func() {
				BeforeEach(func() {
					secretsClient.DeleteReturns(errors.New("boom"))
				})

				It("returns the error", func() {
					Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(MatchError(ContainSubstring("boom")))
				})
			})

			When("the private registry secret does not exist", func() {
				BeforeEach(func() {
					secretsClient.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{
						Group:    "core/v1",
						Resource: "Secret",
					}, "foo"))
				})

				It("succeeds", func() {
					Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				})
			})
		})

		When("deletion of stateful set fails", func() {
			BeforeEach(func() {
				statefulSetClient.DeleteReturns(errors.New("boom"))
			})

			It("should return a meaningful error", func() {
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).
					To(MatchError(ContainSubstring("failed to delete statefulset")))
			})
		})

		When("deletion of stateful set conflicts", func() {
			It("should retry", func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}}, nil)
				statefulSetClient.DeleteReturnsOnCall(0, k8serrors.NewConflict(schema.GroupResource{}, "foo", errors.New("boom")))
				statefulSetClient.DeleteReturnsOnCall(1, nil)
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(Succeed())
				Expect(statefulSetClient.DeleteCallCount()).To(Equal(2))
			})
		})

		When("pdb deletion fails", func() {
			It("returns an error", func() {
				pdbClient.DeleteReturns(errors.New("boom"))

				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})).To(MatchError(ContainSubstring("boom")))
			})
		})

		When("kubernetes fails to list statefulsets", func() {
			BeforeEach(func() {
				statefulSetClient.GetByLRPIdentifierReturns(nil, errors.New("who is this?"))
			})

			It("should return a meaningful error", func() {
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("the statefulSet does not exist", func() {
			BeforeEach(func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
			})

			It("succeeds", func() {
				Expect(statefulSetDesirer.Stop(opi.LRPIdentifier{})).To(Succeed())
			})

			It("logs useful information", func() {
				_ = statefulSetDesirer.Stop(opi.LRPIdentifier{GUID: "missing_guid", Version: "some_version"})
				Expect(logger).To(gbytes.Say("statefulset-does-not-exist.*missing_guid.*some_version"))
			})
		})
	})

	Describe("StopInstance", func() {
		BeforeEach(func() {
			st := []appsv1.StatefulSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baldur-space-foo-34f869d015",
						Namespace: "the-namespace",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32ptr(2),
					},
				},
			}

			statefulSetClient.GetByLRPIdentifierReturns(st, nil)
		})

		It("deletes a pod instance", func() {
			Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 0)).
				To(Succeed())

			Expect(podsClient.DeleteCallCount()).To(Equal(1))

			namespace, name := podsClient.DeleteArgsForCall(0)
			Expect(namespace).To(Equal("the-namespace"))
			Expect(name).To(Equal("baldur-space-foo-34f869d015-0"))
		})

		When("there's an internal K8s error", func() {
			It("should return an error", func() {
				statefulSetClient.GetByLRPIdentifierReturns(nil, errors.New("boom"))
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("the statefulset does not exist", func() {
			It("succeeds", func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "some", Version: "thing"}, 1)).To(Succeed())
			})
		})

		When("the instance index is invalid", func() {
			It("returns an error", func() {
				podsClient.DeleteReturns(errors.New("boom"))
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 42)).
					To(MatchError(eirini.ErrInvalidInstanceIndex))
			})
		})

		When("the instance is already stopped", func() {
			BeforeEach(func() {
				podsClient.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{}, "potato"))
			})

			It("succeeds", func() {
				Expect(statefulSetDesirer.StopInstance(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)).
					To(Succeed())
			})
		})
	})

	Describe("GetInstances", func() {
		BeforeEach(func() {
			statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}}, nil)
		})

		It("should list the correct pods", func() {
			pods := []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-0"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-2"}},
			}
			podsClient.GetByLRPIdentifierReturns(pods, nil)
			eventsClient.GetByPodReturns([]corev1.Event{}, nil)

			_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

			Expect(err).ToNot(HaveOccurred())
			Expect(podsClient.GetByLRPIdentifierCallCount()).To(Equal(1))
			Expect(podsClient.GetByLRPIdentifierArgsForCall(0)).To(Equal(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}))
		})

		It("should return the correct number of instances", func() {
			pods := []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-0"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-2"}},
			}
			podsClient.GetByLRPIdentifierReturns(pods, nil)
			eventsClient.GetByPodReturns([]corev1.Event{}, nil)
			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(3))
		})

		It("should return the correct instances information", func() {
			m := metav1.Unix(123, 0)
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
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
			}

			podsClient.GetByLRPIdentifierReturns(pods, nil)
			eventsClient.GetByPodReturns([]corev1.Event{}, nil)
			instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(1))
			Expect(instances[0].Index).To(Equal(1))
			Expect(instances[0].Since).To(Equal(int64(123000000000)))
			Expect(instances[0].State).To(Equal("RUNNING"))
			Expect(instances[0].PlacementError).To(BeEmpty())
		})

		When("pod list fails", func() {
			It("should return a meaningful error", func() {
				podsClient.GetByLRPIdentifierReturns(nil, errors.New("boom"))

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
				Expect(err).To(MatchError(ContainSubstring("failed to list pods")))
			})
		})

		When("the app does not exist", func() {
			It("should return an error", func() {
				statefulSetClient.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "does-not", Version: "exist"})
				Expect(err).To(Equal(eirini.ErrNotFound))
			})
		})

		When("getting events fails", func() {
			It("should return a meaningful error", func() {
				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podsClient.GetByLRPIdentifierReturns(pods, nil)

				eventsClient.GetByPodReturns(nil, errors.New("I am error"))

				_, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("failed to get events for pod %s", "odin-0"))))
			})
		})

		When("time since creation is not available yet", func() {
			It("should return a default value", func() {
				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podsClient.GetByLRPIdentifierReturns(pods, nil)
				eventsClient.GetByPodReturns([]corev1.Event{}, nil)

				instances, err := statefulSetDesirer.GetInstances(opi.LRPIdentifier{})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(1))
				Expect(instances[0].Since).To(Equal(int64(0)))
			})
		})

		When("pods need too much resources", func() {
			BeforeEach(func() {
				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podsClient.GetByLRPIdentifierReturns(pods, nil)
			})

			When("the cluster has autoscaler", func() {
				BeforeEach(func() {
					eventsClient.GetByPodReturns([]corev1.Event{
						{
							Reason:  "NotTriggerScaleUp",
							Message: "pod didn't trigger scale-up (it wouldn't fit if a new node is added): 1 Insufficient memory",
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

			When("the cluster does not have autoscaler", func() {
				BeforeEach(func() {
					eventsClient.GetByPodReturns([]corev1.Event{
						{
							Reason:  "FailedScheduling",
							Message: "0/3 nodes are available: 3 Insufficient memory.",
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

		When("the StatefulSet was deleted/stopped", func() {
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
				eventsClient.GetByPodReturns([]corev1.Event{
					event1,
					event2,
				}, nil)

				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podsClient.GetByLRPIdentifierReturns(pods, nil)

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
		randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		Expect(err).NotTo(HaveOccurred())

		b[i] = letterBytes[randomNumber.Int64()]
	}

	return string(b)
}

func createLRP(name string, routes []opi.Route) *opi.LRP {
	lastUpdated := randStringBytes()

	return &opi.LRP{
		LRPIdentifier: opi.LRPIdentifier{
			GUID:    "guid_1234",
			Version: "version_1234",
		},
		ProcessType:     "worker",
		AppName:         name,
		AppGUID:         "premium_app_guid_1234",
		SpaceName:       "space-foo",
		SpaceGUID:       "space-guid",
		TargetInstances: 1,
		OrgName:         "org-foo",
		OrgGUID:         "org-guid",
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
