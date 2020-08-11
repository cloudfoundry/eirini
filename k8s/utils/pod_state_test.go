package utils_test

import (
	. "code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("PodState", func() {
	When("the container statuses are not available:", func() {
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
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{},
							},
						},
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{},
							},
						},
					},
					Phase: corev1.PodPending,
				},
			}
			Expect(GetPodState(pod)).To(Equal(opi.PendingState))
		})

		When("the container state is not waiting", func() {
			It("should return 'PENDING' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{},
								},
							},
						},
						Phase: corev1.PodPending,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.PendingState))
			})
		})

		Context("and the image for one of the containers cannot be pulled", func() {
			Context("when there is one container in the pod", func() {
				It("should return 'CRASHED' state", func() {
					pod := corev1.Pod{
						Status: corev1.PodStatus{
							ContainerStatuses: []corev1.ContainerStatus{
								{
									State: corev1.ContainerState{
										Waiting: &corev1.ContainerStateWaiting{},
									},
								},
								{
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
		})

		Context("and kubernetes has backed off from pulling the image of one of the containers", func() {
			It("should return 'CRASHED' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{},
								},
							},
							{
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

	When("the pod phase is Running", func() {
		// this happens when the pod is being shut down, or one of the containers is restarting (e.g. after crash)
		When("and of the containers is Running but not Ready", func() {
			It("should return 'PENDING' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
								Ready: true,
							},
							{
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
								Ready: false,
							},
						},
						Phase: corev1.PodRunning,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.PendingState))
			})
		})
		When("and all containers are Running and Ready", func() {
			It("should return 'RUNNING' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
								Ready: true,
							},
							{
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
								Ready: true,
							},
						},
						Phase: corev1.PodRunning,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.RunningState))
			})
		})

		When("and one of the containers is Waiting with CrashLoopBackOff reason", func() {
			It("should return 'CRASHED' state", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
								Ready: true,
							},
							{
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{
										Message: "failed to start",
										Reason:  "CrashLoopBackOff",
									},
								},
								Ready: false,
							},
						},
						Phase: corev1.PodRunning,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.CrashedState))
			})
		})
	})

	When("the pod phase is Failed:", func() {
		When("and one of the containers is Waiting", func() {
			It("should return 'CRASHED' State", func() {
				pod := corev1.Pod{
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{},
								},
							},
							{
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{},
								},
								Ready: false,
							},
						},
						Phase: corev1.PodFailed,
					},
				}
				Expect(GetPodState(pod)).To(Equal(opi.CrashedState))
			})
		})

		When("and of the containers is Terminated", func() {
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
