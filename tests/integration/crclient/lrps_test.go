package integration_test

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/crclient"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("LRPs", func() {
	var (
		lrp          *eiriniv1.LRP
		lrpsCrClient *crclient.LRPs
	)

	BeforeEach(func() {
		lrp = &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-lrp",
			},
			Spec: eiriniv1.LRPSpec{
				Image:  "eirini/notdora",
				DiskMB: 123,
			},
		}

		var err error
		lrp, err = fixture.EiriniClientset.EiriniV1().LRPs(fixture.Namespace).Create(context.Background(), lrp, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		lrpsCrClient = crclient.NewLRPs(fixture.RuntimeClient)
	})

	Describe("GetLRP", func() {
		It("gets an LRP by namespace and name", func() {
			actualLRP, err := lrpsCrClient.GetLRP(ctx, fixture.Namespace, lrp.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRP).To(Equal(lrp))
		})

		When("the LRP doesn't exist", func() {
			It("fails", func() {
				_, err := lrpsCrClient.GetLRP(ctx, fixture.Namespace, "a-non-existing-lrp")
				Expect(err).To(MatchError(ContainSubstring(`"a-non-existing-lrp" not found`)))
			})
		})
	})

	Describe("UpdateLRPStatus", func() {
		It("updates the LRP replicas in status", func() {
			err := lrpsCrClient.UpdateLRPStatus(ctx, lrp, eiriniv1.LRPStatus{
				Replicas: 3,
			})
			Expect(err).NotTo(HaveOccurred())

			updatedLRP, err := lrpsCrClient.GetLRP(ctx, fixture.Namespace, lrp.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedLRP.Status.Replicas).To(Equal(int32(3)))
		})
	})
})
