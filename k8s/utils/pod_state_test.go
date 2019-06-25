package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
)

var _ = Describe("PodState", func() {

	When("the containerstatuses are not available:", func() {
		It("should return 'UNKNOWN' state", func() {
			pod := corev1.Pod{}
			Expect(GetPodState(pod)).To(Equal(opi.UnknownState))
		})
	})

	When("the pod phase is Unknown:", func() {
		It("should return 'UNKNOWN' state", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{}},
					Phase:             corev1.PodUnknown,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.UnknownState))
		})
	})

	When("the pod phase is Pending:", func() {
		It("should return 'PENDING' state", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "It is fine",
							},
						},
					}},
					Phase: corev1.PodPending,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.PendingState))
		})

		Context("and the image cannot be pulled", func() {
			It("should return 'CRASHED' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ErrImagePull",
								},
							},
						},
						},
						Phase: corev1.PodPending,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.CrashedState))
			})
		})

		Context("and kubernetes has backed off from pulling the image", func() {
			It("should return 'CRASHED' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ImagePullBackOff",
								},
							},
						},
						},
						Phase: corev1.PodPending,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.CrashedState))
			})
		})

	})

	When("the pod is Running and not Ready:", func() {
		It("should return 'PENDING' state", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
						Ready: false,
					}},
					Phase: corev1.PodRunning,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.PendingState))
		})
	})

	When("the pod state is Waiting", func() {
		It("should return 'CRASHED' State", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{},
						},
					}},
					Phase: corev1.PodFailed,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.CrashedState))
		})
	})

	When("the pod state is Terminated", func() {
		It("should return 'CRASHED' State", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{},
						},
					}},
					Phase: corev1.PodFailed,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.CrashedState))
		})
	})

	When("the pod state is Running and Ready", func() {
		It("should return 'RUNNING' State", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
						Ready: true,
					}},
					Phase: corev1.PodFailed,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.RunningState))
		})
	})

	When("the pod state cannot be determined", func() {
		It("should return 'UNKNOWN' State", func() {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{},
					}},
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.UnknownState))
		})
	})
})
