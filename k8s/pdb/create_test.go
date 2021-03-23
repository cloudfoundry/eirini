package pdb_test

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/pdb"
	"code.cloudfoundry.org/eirini/k8s/pdb/pdbfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Pdb", func() {
	var (
		creator   *pdb.CreatorDeleter
		k8sClient *pdbfakes.FakeK8sClient
		lrp       *opi.LRP
		ctx       context.Context
	)

	BeforeEach(func() {
		k8sClient = new(pdbfakes.FakeK8sClient)
		creator = pdb.NewCreatorDeleter(k8sClient)

		lrp = &opi.LRP{
			LRPIdentifier: opi.LRPIdentifier{
				GUID:    "guid",
				Version: "version",
			},
			AppName:         "appName",
			SpaceName:       "spaceName",
			TargetInstances: 2,
		}

		ctx = context.Background()
	})

	Describe("Update", func() {
		var updateErr error
		JustBeforeEach(func() {
			updateErr = creator.Update(ctx, "namespace", "name", lrp)
		})

		It("succeeds", func() {
			Expect(updateErr).NotTo(HaveOccurred())
		})

		It("creates a pod disruption budget", func() {
			Expect(k8sClient.CreateCallCount()).To(Equal(1))

			_, pdbNamespace, pdb := k8sClient.CreateArgsForCall(0)
			Expect(pdbNamespace).To(Equal("namespace"))

			Expect(pdb.Name).To(Equal("name"))
			Expect(pdb.Spec.MinAvailable).To(PointTo(Equal(intstr.FromInt(1))))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(stset.LabelGUID, lrp.GUID))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(stset.LabelVersion, lrp.Version))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(stset.LabelSourceType, "APP"))
		})

		When("pod disruption budget creation fails", func() {
			BeforeEach(func() {
				k8sClient.CreateReturns(nil, fmt.Errorf("boom"))
			})

			It("should propagate the error", func() {
				Expect(updateErr).To(MatchError(ContainSubstring("boom")))
			})
		})

		When("the LRP has less than 2 target instances", func() {
			BeforeEach(func() {
				lrp.TargetInstances = 1
			})

			It("does not create but does try to delete pdb", func() {
				Expect(k8sClient.CreateCallCount()).To(BeZero())
				Expect(k8sClient.DeleteCallCount()).To(Equal(1))
			})

			When("there is no PDB already", func() {
				BeforeEach(func() {
					k8sClient.DeleteReturns(k8serrors.NewNotFound(schema.GroupResource{}, "nope"))
				})

				It("succeeds", func() {
					Expect(updateErr).NotTo(HaveOccurred())
				})
			})

			When("deleting the PDB fails", func() {
				BeforeEach(func() {
					k8sClient.DeleteReturns(errors.New("oops"))
				})

				It("returns an error", func() {
					Expect(updateErr).To(MatchError(ContainSubstring("oops")))
				})
			})
		})

		When("the pod distruption budget already exists", func() {
			BeforeEach(func() {
				k8sClient.CreateReturns(nil, k8serrors.NewAlreadyExists(schema.GroupResource{}, "boom"))
			})

			It("succeeds", func() {
				Expect(updateErr).NotTo(HaveOccurred())
			})
		})
	})
})
