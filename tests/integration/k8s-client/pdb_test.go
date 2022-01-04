package integration_test

import (
	"code.cloudfoundry.org/eirini/k8s/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PodDisruptionBudgets", func() {
	var pdbClient *client.PodDisruptionBudget

	BeforeEach(func() {
		pdbClient = client.NewPodDisruptionBudget(fixture.Clientset)
	})

	Describe("Get", func() {
		BeforeEach(func() {
			createPDB(fixture.Namespace, "foo")
			Eventually(func() []policyv1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).ShouldNot(BeEmpty())
		})

		It("can get a PDB by namespace and name", func() {
			foundPDB, err := pdbClient.Get(ctx, fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(foundPDB.Name).To(Equal("foo"))
			Expect(foundPDB.Namespace).To(Equal(fixture.Namespace))
		})
	})

	Describe("Create", func() {
		It("creates a PDB", func() {
			_, err := pdbClient.Create(ctx, fixture.Namespace, &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			pdbs := listPDBs(fixture.Namespace)

			Expect(pdbs).To(HaveLen(1))
			Expect(pdbs[0].Name).To(Equal("foo"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createPDB(fixture.Namespace, "foo")
		})

		It("deletes a PDB", func() {
			Eventually(func() []policyv1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := pdbClient.Delete(ctx, fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []policyv1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).Should(BeEmpty())
		})
	})

	Describe("set owner", func() {
		var (
			pdb   *policyv1.PodDisruptionBudget
			stSet *appsv1.StatefulSet
		)

		BeforeEach(func() {
			stSet = createStatefulSetSpec(fixture.Namespace, "foo", nil, nil)
			stSet.UID = "my-uid"
			stSet.OwnerReferences = []metav1.OwnerReference{}
			pdb = createPDB(fixture.Namespace, "foo")
			Eventually(func() []policyv1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).ShouldNot(BeEmpty())
		})

		It("updates owner info", func() {
			updatedPDB, err := pdbClient.SetOwner(ctx, pdb, stSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedPDB.OwnerReferences).To(HaveLen(1))
			Expect(updatedPDB.OwnerReferences[0].Name).To(Equal("foo"))
			Expect(updatedPDB.OwnerReferences[0].Kind).To(Equal("StatefulSet"))
			Expect(string(updatedPDB.OwnerReferences[0].UID)).To(Equal("my-uid"))
		})
	})
})
