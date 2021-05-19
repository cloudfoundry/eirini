package reconciler_test

import (
	"context"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	eiriniv1scheme "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned/scheme"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("reconciler.LRP", func() {
	var (
		logger         *lagertest.TestLogger
		lrpsCrClient   *reconcilerfakes.FakeLRPsCrClient
		workloadClient *reconcilerfakes.FakeLRPWorkloadCLient
		scheme         *runtime.Scheme
		lrpreconciler  *reconciler.LRP
		resultErr      error
	)

	BeforeEach(func() {
		lrpsCrClient = new(reconcilerfakes.FakeLRPsCrClient)
		workloadClient = new(reconcilerfakes.FakeLRPWorkloadCLient)
		logger = lagertest.NewTestLogger("lrp-reconciler")
		scheme = eiriniv1scheme.Scheme
		lrpreconciler = reconciler.NewLRP(logger, lrpsCrClient, workloadClient, scheme)

		lrpsCrClient.GetLRPReturns(&eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-lrp",
				Namespace: "some-ns",
			},
			Spec: eiriniv1.LRPSpec{
				GUID:        "the-lrp-guid",
				Version:     "the-lrp-version",
				Command:     []string{"ls", "-la"},
				Instances:   10,
				ProcessType: "web",
				AppName:     "the-app",
				AppGUID:     "the-app-guid",
				OrgName:     "the-org",
				OrgGUID:     "the-org-guid",
				SpaceName:   "the-space",
				SpaceGUID:   "the-space-guid",
				Image:       "eirini/dorini",
				Env: map[string]string{
					"FOO": "BAR",
				},
				Ports:       []int32{8080, 9090},
				MemoryMB:    1024,
				DiskMB:      512,
				CPUWeight:   128,
				LastUpdated: "now",
				Sidecars: []eiriniv1.Sidecar{
					{
						Name:     "hello-sidecar",
						Command:  []string{"sh", "-c", "echo hello"},
						MemoryMB: 8,
						Env: map[string]string{
							"SIDE": "BUS",
						},
					},
					{
						Name:     "bye-sidecar",
						Command:  []string{"sh", "-c", "echo bye"},
						MemoryMB: 16,
						Env: map[string]string{
							"SIDE": "CAR",
						},
					},
				},
				VolumeMounts: []eiriniv1.VolumeMount{
					{
						MountPath: "/path/to/mount",
						ClaimName: "claim-q1",
					},
					{
						MountPath: "/path/in/the/other/direction",
						ClaimName: "claim-c2",
					},
				},
				Health: eiriniv1.Healthcheck{
					Type:      "http",
					Port:      9090,
					Endpoint:  "/heath",
					TimeoutMs: 80,
				},
				UserDefinedAnnotations: map[string]string{
					"user-annotaions.io": "yes",
				},
			},
		}, nil)

		workloadClient.GetReturns(nil, eirini.ErrNotFound)
	})

	JustBeforeEach(func() {
		_, resultErr = lrpreconciler.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "some-ns",
				Name:      "app",
			},
		})
	})

	It("creates a statefulset for each CRD", func() {
		Expect(resultErr).NotTo(HaveOccurred())

		Expect(workloadClient.UpdateCallCount()).To(Equal(0))
		Expect(workloadClient.DesireCallCount()).To(Equal(1))

		_, ns, lrp, _ := workloadClient.DesireArgsForCall(0)
		Expect(ns).To(Equal("some-ns"))
		Expect(lrp.GUID).To(Equal("the-lrp-guid"))
		Expect(lrp.Version).To(Equal("the-lrp-version"))
		Expect(lrp.Command).To(ConsistOf("ls", "-la"))
		Expect(lrp.TargetInstances).To(Equal(10))
		Expect(lrp.PrivateRegistry).To(BeNil())
		Expect(lrp.ProcessType).To(Equal("web"))
		Expect(lrp.AppName).To(Equal("the-app"))
		Expect(lrp.AppGUID).To(Equal("the-app-guid"))
		Expect(lrp.OrgName).To(Equal("the-org"))
		Expect(lrp.OrgGUID).To(Equal("the-org-guid"))
		Expect(lrp.SpaceName).To(Equal("the-space"))
		Expect(lrp.SpaceGUID).To(Equal("the-space-guid"))
		Expect(lrp.Image).To(Equal("eirini/dorini"))
		Expect(lrp.Env).To(Equal(map[string]string{
			"FOO": "BAR",
		}))
		Expect(lrp.Ports).To(Equal([]int32{8080, 9090}))
		Expect(lrp.MemoryMB).To(Equal(int64(1024)))
		Expect(lrp.DiskMB).To(Equal(int64(512)))
		Expect(lrp.CPUWeight).To(Equal(uint8(128)))
		Expect(lrp.LastUpdated).To(Equal("now"))
		Expect(lrp.Sidecars).To(Equal([]api.Sidecar{
			{
				Name:     "hello-sidecar",
				Command:  []string{"sh", "-c", "echo hello"},
				MemoryMB: 8,
				Env: map[string]string{
					"SIDE": "BUS",
				},
			},
			{
				Name:     "bye-sidecar",
				Command:  []string{"sh", "-c", "echo bye"},
				MemoryMB: 16,
				Env: map[string]string{
					"SIDE": "CAR",
				},
			},
		}))
		Expect(lrp.VolumeMounts).To(Equal([]api.VolumeMount{
			{
				MountPath: "/path/to/mount",
				ClaimName: "claim-q1",
			},
			{
				MountPath: "/path/in/the/other/direction",
				ClaimName: "claim-c2",
			},
		}))
		Expect(lrp.Health).To(Equal(api.Healthcheck{
			Type:      "http",
			Port:      9090,
			Endpoint:  "/heath",
			TimeoutMs: 80,
		}))
		Expect(lrp.UserDefinedAnnotations).To(Equal(map[string]string{
			"user-annotaions.io": "yes",
		}))
	})

	It("sets an owner reference in the statefulset", func() {
		Expect(resultErr).NotTo(HaveOccurred())

		Expect(workloadClient.DesireCallCount()).To(Equal(1))
		_, _, _, setOwnerFns := workloadClient.DesireArgsForCall(0)
		Expect(setOwnerFns).To(HaveLen(1))
		setOwnerFn := setOwnerFns[0]

		st := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns"}}
		Expect(setOwnerFn(st)).To(Succeed())
		Expect(st.ObjectMeta.OwnerReferences).To(HaveLen(1))
		Expect(st.ObjectMeta.OwnerReferences[0].Kind).To(Equal("LRP"))
		Expect(st.ObjectMeta.OwnerReferences[0].Name).To(Equal("some-lrp"))
	})

	It("does not update the LRP CR", func() {
		Expect(lrpsCrClient.UpdateLRPStatusCallCount()).To(BeZero())
	})

	When("the statefulset for the LRP already exists", func() {
		BeforeEach(func() {
			workloadClient.GetReturns(&api.LRP{
				LRPIdentifier: api.LRPIdentifier{
					GUID:    "the-lrp-guid",
					Version: "the-lrp-version",
				},
			}, nil)

			workloadClient.GetStatusReturns(eiriniv1.LRPStatus{
				Replicas: 9,
			}, nil)
		})

		It("updates the CR status accordingly", func() {
			Expect(resultErr).NotTo(HaveOccurred())

			Expect(lrpsCrClient.UpdateLRPStatusCallCount()).To(Equal(1))
			_, actualLrp, actualLrpStatus := lrpsCrClient.UpdateLRPStatusArgsForCall(0)
			Expect(actualLrp.Name).To(Equal("some-lrp"))
			Expect(actualLrpStatus.Replicas).To(Equal(int32(9)))
		})

		When("gettting the LRP status fails", func() {
			BeforeEach(func() {
				workloadClient.GetStatusReturns(eiriniv1.LRPStatus{}, errors.New("boom"))
			})

			It("does not update the statefulset status", func() {
				Expect(resultErr).To(MatchError(ContainSubstring("boom")))
				Expect(workloadClient.UpdateCallCount()).To(Equal(1))
				Expect(lrpsCrClient.UpdateLRPStatusCallCount()).To(BeZero())
			})
		})
	})

	When("private registry credentials are specified in the LRP CRD", func() {
		BeforeEach(func() {
			lrpsCrClient.GetLRPReturns(&eiriniv1.LRP{
				Spec: eiriniv1.LRPSpec{
					Image: "private-registry.com:5000/repo/app-image:latest",
					PrivateRegistry: &eiriniv1.PrivateRegistry{
						Username: "docker-user",
						Password: "docker-password",
					},
				},
			}, nil)
		})

		It("configures a private registry", func() {
			_, _, lrp, _ := workloadClient.DesireArgsForCall(0)
			privateRegistry := lrp.PrivateRegistry
			Expect(privateRegistry).NotTo(BeNil())
			Expect(privateRegistry.Username).To(Equal("docker-user"))
			Expect(privateRegistry.Password).To(Equal("docker-password"))
			Expect(privateRegistry.Server).To(Equal("private-registry.com"))
		})

		When("the image URL does not contain an image registry host", func() {
			BeforeEach(func() {
				lrpsCrClient.GetLRPReturns(&eiriniv1.LRP{
					Spec: eiriniv1.LRPSpec{
						Image: "eirini/dorini",
						PrivateRegistry: &eiriniv1.PrivateRegistry{
							Username: "docker-user",
							Password: "docker-password",
						},
					},
				}, nil)
			})

			It("configures the private registry server with the dockerhub host", func() {
				_, _, lrp, _ := workloadClient.DesireArgsForCall(0)
				Expect(lrp.PrivateRegistry.Server).To(Equal("index.docker.io/v1/"))
			})
		})
	})

	When("a CRD for this app already exists", func() {
		BeforeEach(func() {
			workloadClient.GetReturns(nil, nil)
		})

		It("updates it", func() {
			Expect(resultErr).NotTo(HaveOccurred())

			Expect(workloadClient.UpdateCallCount()).To(Equal(1))
			_, lrp := workloadClient.UpdateArgsForCall(0)
			Expect(lrp.TargetInstances).To(Equal(10))
		})
	})

	When("the LRP doesn't exist", func() {
		BeforeEach(func() {
			lrpsCrClient.GetLRPReturns(nil, apierrors.NewNotFound(schema.GroupResource{}, "my-lrp"))
		})

		It("does not return an error", func() {
			Expect(resultErr).NotTo(HaveOccurred())
		})
	})

	When("the controller client fails to get the CRD", func() {
		BeforeEach(func() {
			lrpsCrClient.GetLRPReturns(nil, errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError(ContainSubstring("boom")))
		})
	})

	When("the workload client fails to get the app", func() {
		BeforeEach(func() {
			workloadClient.GetReturns(nil, errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError("failed to get lrp: boom"))
		})
	})

	When("the workload client fails to desire the app", func() {
		BeforeEach(func() {
			workloadClient.DesireReturns(errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError("failed to desire lrp: boom"))
		})
	})

	When("the workload client fails to update the app", func() {
		BeforeEach(func() {
			workloadClient.GetReturns(nil, nil)
			workloadClient.UpdateReturns(errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError(ContainSubstring("boom")))
		})
	})
})
