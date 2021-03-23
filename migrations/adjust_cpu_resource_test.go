package migrations_test

import (
	"errors"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/migrations"
	"code.cloudfoundry.org/eirini/migrations/migrationsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("AdjustCpuResource", func() {
	var (
		migrator         migrations.AdjustCPURequest
		stSet            runtime.Object
		cpuRequestSetter *migrationsfakes.FakeCPURequestSetter
		migrationErr     error
	)

	BeforeEach(func() {
		stSet = &appsv1.StatefulSet{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					stset.AnnotationOriginalRequest: `{"cpu_weight":123}`,
				},
			},
		}

		cpuRequestSetter = new(migrationsfakes.FakeCPURequestSetter)
		migrator = migrations.NewAdjustCPURequest(cpuRequestSetter)
	})

	JustBeforeEach(func() {
		migrationErr = migrator.Apply(ctx, stSet)
	})

	It("updates the cpu resource request to match that in the stateful set original request annotation", func() {
		Expect(migrationErr).NotTo(HaveOccurred())
		Expect(cpuRequestSetter.SetCPURequestCallCount()).To(Equal(1))
		_, actualStset, actualRequestValue := cpuRequestSetter.SetCPURequestArgsForCall(0)
		Expect(actualStset).To(Equal(stSet))
		Expect(actualRequestValue.MilliValue()).To(Equal(int64(123)))
	})

	When("a non-statefulset object is received", func() {
		BeforeEach(func() {
			stSet = &appsv1.ReplicaSet{}
		})

		It("errors", func() {
			Expect(migrationErr).To(MatchError("expected *v1.StatefulSet, got: *v1.ReplicaSet"))
		})
	})

	When("original request annotation is malformed json", func() {
		BeforeEach(func() {
			stSet = &appsv1.StatefulSet{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						stset.AnnotationOriginalRequest: "non-json",
					},
				},
			}
		})

		It("errors", func() {
			Expect(migrationErr).To(MatchError(ContainSubstring("not valid json")))
		})
	})

	When("the cpu request setter fails", func() {
		BeforeEach(func() {
			cpuRequestSetter.SetCPURequestReturns(nil, errors.New("boom"))
		})

		It("errors", func() {
			Expect(migrationErr).To(MatchError(ContainSubstring("boom")))
		})
	})
})
