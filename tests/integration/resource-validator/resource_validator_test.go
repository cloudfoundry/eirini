package resource_validator_test

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ResourceValidator", func() {
	var (
		lrpName         string
		lrp             *eiriniv1.LRP
		validationError error
	)

	BeforeEach(func() {
		lrpName = tests.GenerateGUID()

		var err error
		lrp, err = fixture.EiriniClientset.EiriniV1().LRPs(fixture.Namespace).Create(
			context.Background(),
			&eiriniv1.LRP{
				ObjectMeta: metav1.ObjectMeta{
					Name: lrpName,
				},
				Spec: eiriniv1.LRPSpec{
					GUID:                   tests.GenerateGUID(),
					Version:                tests.GenerateGUID(),
					Image:                  "eirini/dorini",
					AppGUID:                "the-app-guid",
					AppName:                "k-2so",
					SpaceName:              "s",
					OrgName:                "o",
					Env:                    map[string]string{"FOO": "BAR"},
					MemoryMB:               256,
					DiskMB:                 256,
					CPUWeight:              10,
					Instances:              1,
					LastUpdated:            "a long time ago in a galaxy far, far away",
					Ports:                  []int32{8080},
					VolumeMounts:           []eiriniv1.VolumeMount{{MountPath: "path", ClaimName: "name"}},
					UserDefinedAnnotations: map[string]string{},
					AppRoutes:              []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
				},
			},
			metav1.CreateOptions{},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		lrp, validationError = fixture.EiriniClientset.EiriniV1().LRPs(fixture.Namespace).Update(context.Background(), lrp, metav1.UpdateOptions{})
	})

	When("nothing is updated", func() {
		It("allows the change", func() {
			Expect(validationError).NotTo(HaveOccurred())
		})
	})

	When("the instance count is updated", func() {
		BeforeEach(func() {
			lrp.Spec.Instances = 2
		})

		It("allows the change", func() {
			Expect(validationError).NotTo(HaveOccurred())
		})
	})

	When("the image is updated", func() {
		BeforeEach(func() {
			lrp.Spec.Image = "busybox"
		})
		It("allows the change", func() {
			Expect(validationError).NotTo(HaveOccurred())
		})
	})

	When("the app routes are updated", func() {
		BeforeEach(func() {
			lrp.Spec.AppRoutes[0].Port = 1234
		})

		It("allows the change", func() {
			Expect(validationError).NotTo(HaveOccurred())
		})
	})

	When("an immutable field is updated", func() {
		BeforeEach(func() {
			lrp.Spec.VolumeMounts[0].MountPath = "foo"
			lrp.Spec.VolumeMounts[0].ClaimName = "bar"
		})

		It("disallows the change", func() {
			Expect(validationError).To(MatchError(ContainSubstring("Changing immutable fields not allowed: VolumeMounts")))
		})
	})
})
