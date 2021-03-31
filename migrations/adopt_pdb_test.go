package migrations_test

import (
	"errors"

	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("PDB Adoption", func() {
	var (
		adoptPDBMigration migrations.AdoptPDB
		stSet             runtime.Object
		pdb               *policyv1beta1.PodDisruptionBudget
		pdbClient         *migrationsfakes.FakePDBClient
		migrateErr        error
	)

	BeforeEach(func() {
		pdbClient = new(migrationsfakes.FakePDBClient)
		adoptPDBMigration = migrations.NewAdoptPDB(pdbClient)

		var two int32 = 2
		stSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-stateful-set",
				Namespace: "my-namespace",
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: &two,
				Template: corev1.PodTemplateSpec{},
			},
		}

		pdb = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-stateful-set",
				Namespace: "my-namespace",
			},
		}

		pdbClient.GetReturns(pdb, nil)
	})

	JustBeforeEach(func() {
		migrateErr = adoptPDBMigration.Apply(ctx, stSet)
	})

	It("succeeds", func() {
		Expect(migrateErr).NotTo(HaveOccurred())
	})

	It("requests a PDB matching the stateful set", func() {
		Expect(pdbClient.GetCallCount()).To(Equal(1))
		_, actualNS, actualName := pdbClient.GetArgsForCall(0)
		Expect(actualNS).To(Equal("my-namespace"))
		Expect(actualName).To(Equal("my-stateful-set"))
	})

	It("requests setting the owner of the PDB to the stateful set", func() {
		Expect(pdbClient.SetOwnerCallCount()).To(Equal(1))
		_, actualPDB, actualOwner := pdbClient.SetOwnerArgsForCall(0)
		Expect(actualPDB).To(Equal(pdb))
		Expect(actualOwner).To(Equal(stSet))
	})

	When("the stateful set only has a single instance", func() {
		BeforeEach(func() {
			var one int32 = 1
			statefulSet, ok := stSet.(*appsv1.StatefulSet)
			Expect(ok).To(BeTrue())
			statefulSet.Spec.Replicas = &one
		})

		It("does not migrate any PDB as there won't be one", func() {
			Expect(migrateErr).NotTo(HaveOccurred())
			Expect(pdbClient.GetCallCount()).To(BeZero())
			Expect(pdbClient.SetOwnerCallCount()).To(BeZero())
		})
	})

	When("a non-statefulset object is received", func() {
		BeforeEach(func() {
			stSet = &appsv1.ReplicaSet{}
		})

		It("errors", func() {
			Expect(migrateErr).To(MatchError("expected *v1.StatefulSet, got: *v1.ReplicaSet"))
		})
	})

	When("getting the PDB fails", func() {
		BeforeEach(func() {
			pdbClient.GetReturns(nil, errors.New("no pdb for you"))
		})

		It("bubbles up the error", func() {
			Expect(migrateErr).To(MatchError(ContainSubstring("no pdb for you")))
		})
	})

	When("setting the PDB owner fails", func() {
		BeforeEach(func() {
			pdbClient.SetOwnerReturns(nil, errors.New("can't set owner"))
		})

		It("bubbles up the error", func() {
			Expect(migrateErr).To(MatchError(ContainSubstring("can't set owner")))
		})
	})
})
