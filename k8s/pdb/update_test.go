package pdb_test

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/pdb"
	"code.cloudfoundry.org/eirini/k8s/pdb/pdbfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("PDB", func() {
	var (
		creator   *pdb.Updater
		k8sClient *pdbfakes.FakeK8sClient
		stSet     *appsv1.StatefulSet
		lrp       *api.LRP
		ctx       context.Context
	)

	BeforeEach(func() {
		k8sClient = new(pdbfakes.FakeK8sClient)
		creator = pdb.NewUpdater(k8sClient)

		stSet = &appsv1.StatefulSet{
			ObjectMeta: v1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				UID:       "uid",
			},
		}

		lrp = &api.LRP{
			LRPIdentifier: api.LRPIdentifier{
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
			updateErr = creator.Update(ctx, stSet, lrp)
		})

		It("succeeds", func() {
			Expect(updateErr).NotTo(HaveOccurred())
		})

		It("creates a pod disruption budget", func() {
			Expect(k8sClient.CreateCallCount()).To(Equal(1))

			_, pdbNamespace, pdb := k8sClient.CreateArgsForCall(0)
			Expect(pdbNamespace).To(Equal("namespace"))

			Expect(pdb.Name).To(Equal("name"))
			Expect(pdb.Spec.MinAvailable).To(PointTo(Equal(intstr.FromString("50%"))))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(stset.LabelGUID, lrp.GUID))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(stset.LabelVersion, lrp.Version))
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(stset.LabelSourceType, "APP"))
			Expect(pdb.OwnerReferences).To(HaveLen(1))
			Expect(pdb.OwnerReferences[0].Name).To(Equal(stSet.Name))
			Expect(pdb.OwnerReferences[0].UID).To(Equal(stSet.UID))
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
