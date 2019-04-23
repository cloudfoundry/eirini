package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"

	. "code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
)

var _ = Describe("PodState", func() {

	When("the containerstatuses are not available:", func() {
		It("should return 'UNKNOWN' state", func() {
			pod := v1.Pod{}
			Expect(GetPodState(pod)).To(Equal(opi.UnknownState))
		})
	})

	When("the pod phase is Unknown:", func() {
		It("should return 'UNKNOWN' state", func() {
			pod := v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{{}},
					Phase:             v1.PodUnknown,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.UnknownState))
		})
	})
	When("the pod phase is Pending:", func() {
		It("should return 'PENDING' state", func() {
			pod := v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{{}},
					Phase:             v1.PodPending,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.PendingState))
		})
	})
	When("the pod is Running and not Ready:", func() {
		It("should return 'PENDING' state", func() {
			pod := v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{{
						State: v1.ContainerState{
							Running: &v1.ContainerStateRunning{},
						},
						Ready: false,
					}},
					Phase: v1.PodRunning,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.PendingState))
		})
	})
})
