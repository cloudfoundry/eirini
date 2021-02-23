package eirini_controller_test

import (
	"context"
	"encoding/json"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("EiriniController", func() {
	var (
		lrpName    string
		lrpGUID    string
		lrpVersion string
		lrp        *eiriniv1.LRP
	)

	BeforeEach(func() {
		lrpName = tests.GenerateGUID()
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()

		lrp = &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name: lrpName,
			},
			Spec: eiriniv1.LRPSpec{
				GUID:                   lrpGUID,
				Version:                lrpVersion,
				Image:                  "eirini/dorini",
				AppGUID:                "the-app-guid",
				AppName:                "k-2so",
				SpaceName:              "s",
				OrgName:                "o",
				Env:                    map[string]string{"FOO": "BAR"},
				MemoryMB:               256,
				DiskMB:                 256,
				CPUWeight:              10,
				Instances:              2,
				LastUpdated:            "a long time ago in a galaxy far, far away",
				Ports:                  []int32{8080},
				VolumeMounts:           []eiriniv1.VolumeMount{},
				UserDefinedAnnotations: map[string]string{},
				AppRoutes:              []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
			},
		}
	})

	JustBeforeEach(func() {
		var err error
		lrp, err = fixture.EiriniClientset.
			EiriniV1().
			LRPs(fixture.Namespace).
			Create(context.Background(), lrp, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates a default PDB", func() {
		pdb := tests.GetPDB(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
		Expect(pdb.Spec.MinAvailable).To(PointTo(Equal(intstr.FromInt(1))))
		Expect(pdb.Spec.MaxUnavailable).To(BeNil())
	})

	When("the LRP has a single instance", func() {
		BeforeEach(func() {
			lrp.Spec.Instances = 1
		})

		It("does not create a PDB", func() {
			Consistently(func() ([]v1beta1.PodDisruptionBudget, error) {
				return tests.GetPDBItems(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			}, "10s").Should(BeEmpty())
		})
	})

	When("scaling the LRP down to one instance", func() {
		JustBeforeEach(func() {
			Expect(tests.GetPDB(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)).NotTo(BeNil())

			patch := []struct {
				Op    string `json:"op"`
				Path  string `json:"path"`
				Value int    `json:"value"`
			}{{Op: "replace", Path: "/spec/instances", Value: 1}}

			patchBytes, err := json.Marshal(patch)
			Expect(err).NotTo(HaveOccurred())

			_, err = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Patch(context.Background(), lrpName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the PDB", func() {
			Eventually(func() ([]v1beta1.PodDisruptionBudget, error) {
				return tests.GetPDBItems(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			}).Should(BeEmpty())
		})
	})

	When("controller config has minAvailable set", func() {
		BeforeEach(func() {
			config.Properties.DefaultMinAvailableInstances = "20%"
		})

		It("creates apdb with the configured value", func() {
			pdb := tests.GetPDB(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			Expect(pdb.Spec.MinAvailable).To(PointTo(Equal(intstr.FromString("20%"))))
			Expect(pdb.Spec.MaxUnavailable).To(BeNil())
		})
	})
})
